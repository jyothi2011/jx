package prow_test

import (
	"github.com/jenkins-x/jx/pkg/prow"
	prowconfig "github.com/jenkins-x/jx/pkg/prow/config"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	"testing"

	"github.com/ghodss/yaml"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/plugins"
)

type TestOptions struct {
	prow.Options
}

func (o *TestOptions) Setup() {
	o.Options = prow.Options{
		KubeClient: testclient.NewSimpleClientset(),
		Repos:      []string{"test/repo"},
		NS:         "test",
		DraftPack:  "maven",
	}
}

func TestProwConfigEnvironment(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	err := o.AddProwConfig()
	assert.NoError(t, err)
}

func TestProwPlugins(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	err := o.AddProwPlugins()
	assert.NoError(t, err)
}

func TestMergeProwConfigEnvironment(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	prowConfig := &config.Config{}
	prowConfig.LogLevel = "debug"

	c, err := yaml.Marshal(prowConfig)
	assert.NoError(t, err)

	data := make(map[string]string)
	data[prow.ProwConfigFilename] = string(c)

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: prow.ProwConfigMapName,
		},
		Data: data,
	}

	_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Create(cm)
	assert.NoError(t, err)

	err = o.AddProwConfig()
	assert.NoError(t, err)

	cm, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)
	assert.Equal(t, "debug", prowConfig.LogLevel)
	assert.NotEmpty(t, prowConfig.Presubmits["test/repo"])

}

func TestMergeProwPlugin(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	pluginConfig := &plugins.Configuration{}
	pluginConfig.Welcome = []plugins.Welcome{{MessageTemplate: "okey dokey"}}

	c, err := yaml.Marshal(pluginConfig)
	assert.NoError(t, err)

	data := make(map[string]string)
	data[prow.ProwPluginsFilename] = string(c)

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: prow.ProwPluginsConfigMapName,
		},
		Data: data,
	}

	_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Create(cm)
	assert.NoError(t, err)

	err = o.AddProwPlugins()
	assert.NoError(t, err)

	cm, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwPluginsConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	yaml.Unmarshal([]byte(cm.Data[prow.ProwPluginsFilename]), &pluginConfig)
	assert.Equal(t, "okey dokey", pluginConfig.Welcome[0].MessageTemplate)
	assert.Equal(t, "test/repo", pluginConfig.Approve[0].Repos[0])

}

func TestAddProwPlugin(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	o.Repos = append(o.Repos, "test/repo2")

	err := o.AddProwPlugins()
	assert.NoError(t, err)

	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwPluginsConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	pluginConfig := &plugins.Configuration{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwPluginsFilename]), &pluginConfig)

	assert.Equal(t, "test/repo", pluginConfig.Approve[0].Repos[0])
	assert.Equal(t, "test/repo2", pluginConfig.Approve[1].Repos[0])

}

func TestAddProwConfig(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	o.Repos = append(o.Repos, "test/repo2")

	err := o.AddProwConfig()
	assert.NoError(t, err)

	prowConfig, err := getProwConfig(t, o)
	assert.NoError(t, err)

	for _, repo := range []string{"test/repo", "test/repo2"} {
		assertInPluginConfig(t, prowConfig, repo, true)
	}
}

func TestRemoveProwConfig(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"
	o.Repos = append(o.Repos, "test/repo2")
	err := o.AddProwConfig()
	assert.NoError(t, err)

	// Remove test/repo (created in o.Setup())
	o.Repos = []string{"test/repo"}
	err = o.RemoveProwConfig()
	assert.NoError(t, err, "errored removing prow config")

	prowConfig, err := getProwConfig(t, o)
	assert.NoError(t, err)

	// test/repo should NOT be in the plugin config, but test/repo2 should
	assertInPluginConfig(t, prowConfig, "test/repo", false)
	assertInPluginConfig(t, prowConfig, "test/repo2", true)
}

// make sure that rerunning addProwConfig replaces any modified changes in the configmap
func TestReplaceProwConfig(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Kind = prowconfig.Environment
	o.EnvironmentNamespace = "jx-staging"

	err := o.AddProwConfig()
	assert.NoError(t, err)

	// now modify the cm
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	prowConfig := &config.Config{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)

	assert.Equal(t, 1, len(prowConfig.Tide.Queries[0].Repos))
	assert.Equal(t, 2, len(prowConfig.Tide.Queries[1].Repos))

	p := prowConfig.Presubmits["test/repo"]
	p[0].Agent = "foo"

	configYAML, err := yaml.Marshal(&prowConfig)
	assert.NoError(t, err)

	data := make(map[string]string)
	data[prow.ProwConfigFilename] = string(configYAML)
	cm = &v1.ConfigMap{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: prow.ProwConfigMapName,
		},
	}

	_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Update(cm)

	// ensure the value was modified
	cm, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	prowConfig = &config.Config{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)

	p = prowConfig.Presubmits["test/repo"]
	assert.Equal(t, "foo", p[0].Agent)

	// generate the prow config again
	err = o.AddProwConfig()
	assert.NoError(t, err)

	// assert value is reset
	cm, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	prowConfig = &config.Config{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)

	assert.Equal(t, 1, len(prowConfig.Tide.Queries[0].Repos))
	assert.Equal(t, 2, len(prowConfig.Tide.Queries[1].Repos))

	p = prowConfig.Presubmits["test/repo"]
	assert.Equal(t, "knative-build", p[0].Agent)

	// add test/repo2
	o.Options.Repos = []string{"test/repo2"}
	o.Kind = prowconfig.Application

	err = o.AddProwConfig()
	assert.NoError(t, err)

	cm, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	prowConfig = &config.Config{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)

	assert.Equal(t, 2, len(prowConfig.Tide.Queries[0].Repos))
	assert.Equal(t, 2, len(prowConfig.Tide.Queries[1].Repos))

	// add test/repo3
	o.Options.Repos = []string{"test/repo3"}
	o.Kind = prowconfig.Application

	err = o.AddProwConfig()
	assert.NoError(t, err)

	cm, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	prowConfig = &config.Config{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)

	assert.Equal(t, 3, len(prowConfig.Tide.Queries[0].Repos))
	assert.Equal(t, 2, len(prowConfig.Tide.Queries[1].Repos))
}

func TestGetReleaseJobs(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Options.Repos = []string{"test/repo"}
	o.Kind = prowconfig.Application

	err := o.AddProwConfig()
	assert.NoError(t, err)

	// now lets get the release job
	names, err := o.GetReleaseJobs()

	assert.NotEmpty(t, names, err)
	assert.Equal(t, "test/repo/master", names[0])

}

func TestGetPostSubmitJob(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()
	o.Options.Repos = []string{"test/repo"}
	o.Kind = prowconfig.Application

	err := o.AddProwConfig()
	assert.NoError(t, err)

	// now lets get the release job
	job, err := o.GetPostSubmitJob("test", "repo", "master")

	assert.NotEmpty(t, job.Name, "job name is empty")
	assert.Equal(t, "release", job.Name)
}

func getProwConfig(t *testing.T, o TestOptions) (*config.Config, error) {
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(prow.ProwConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	prowConfig := &config.Config{}
	yaml.Unmarshal([]byte(cm.Data[prow.ProwConfigFilename]), &prowConfig)
	return prowConfig, err
}

func assertInPluginConfig(t *testing.T, prowConfig *config.Config, repo string, shouldBeInConfig bool) {
	org, r, _ := util.GetRemoteAndRepo(repo)
	if shouldBeInConfig {
		assert.NotEmpty(t, prowConfig.Presubmits[repo])
		assert.NotEmpty(t, prowConfig.Postsubmits[repo])
		assert.NotEmpty(t, prowConfig.BranchProtection.Orgs[org].Repos[r])
		assert.Contains(t, prowConfig.Tide.Queries[1].Repos, repo)
	} else {
		assert.Empty(t, prowConfig.Presubmits[repo])
		assert.Empty(t, prowConfig.Postsubmits[repo])
		assert.Empty(t, prowConfig.BranchProtection.Orgs[org].Repos[r])
		assert.NotContains(t, prowConfig.Tide.Queries[1].Repos, repo)
	}
}
