// Package buildnum contains stuff to do with generating build numbers.
package buildnum

import "github.com/jenkins-x/jx/pkg/kube"

// BuildNumberIssuer generates build numbers for activities.
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/buildnum BuildNumberIssuer -o mocks/build_num.go --generate-matchers
type BuildNumberIssuer interface {

	//Generate the next build number for the supplied pipeline.
	//Returns the build number, or the error that occurred.
	NextBuildNumber(pipeline kube.PipelineID) (string, error)
}
