package rehearse

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"testing"

	"k8s.io/api/core/v1"

	"k8s.io/test-infra/prow/config"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/diff"
)

func makeTestingPresubmit(env []v1.EnvVar) *config.Presubmit {
	return &config.Presubmit{
		JobBase: config.JobBase{
			Name: "test-job-name",
			Spec: &v1.PodSpec{
				Containers: []v1.Container{
					{Env: env},
				},
			},
		},
	}
}

type fakeCiopConfig struct {
	fakeFiles map[string]string
}

func (c *fakeCiopConfig) Load(repo, configFile string) (string, error) {
	fullPath := filepath.Join(repo, configFile)
	content, ok := c.fakeFiles[fullPath]
	if ok {
		return content, nil
	}

	return "", fmt.Errorf("no such fake file")
}

func makeCMReference(cmName, key string) *v1.EnvVarSource {
	return &v1.EnvVarSource{
		ConfigMapKeyRef: &v1.ConfigMapKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: cmName,
			},
			Key: key,
		},
	}
}

func TestInlineCiopConfig(t *testing.T) {
	testTargetRepo := "org/repo"
	testLogger := logrus.New()

	testCases := []struct {
		description   string
		sourceEnv     []v1.EnvVar
		configs       *fakeCiopConfig
		expectedEnv   []v1.EnvVar
		expectedError bool
	}{{
		description: "empty env -> no changes",
		configs:     &fakeCiopConfig{},
	}, {
		description: "no Env.ValueFrom -> no changes",
		sourceEnv:   []v1.EnvVar{{Name: "T", Value: "V"}},
		configs:     &fakeCiopConfig{},
		expectedEnv: []v1.EnvVar{{Name: "T", Value: "V"}},
	}, {
		description: "no Env.ValueFrom.ConfigMapKeyRef -> no changes",
		sourceEnv:   []v1.EnvVar{{Name: "T", ValueFrom: &v1.EnvVarSource{ResourceFieldRef: &v1.ResourceFieldSelector{}}}},
		configs:     &fakeCiopConfig{},
		expectedEnv: []v1.EnvVar{{Name: "T", ValueFrom: &v1.EnvVarSource{ResourceFieldRef: &v1.ResourceFieldSelector{}}}},
	}, {
		description: "CM reference but not ci-operator-configs -> no changes",
		sourceEnv:   []v1.EnvVar{{Name: "T", ValueFrom: makeCMReference("test-cm", "key")}},
		configs:     &fakeCiopConfig{},
		expectedEnv: []v1.EnvVar{{Name: "T", ValueFrom: makeCMReference("test-cm", "key")}},
	}, {
		description: "CM reference to ci-operator-configs -> cm content inlined",
		sourceEnv:   []v1.EnvVar{{Name: "T", ValueFrom: makeCMReference(ciOperatorConfigsCMName, "filename")}},
		configs:     &fakeCiopConfig{fakeFiles: map[string]string{"org/repo/filename": "ciopConfigContent"}},
		expectedEnv: []v1.EnvVar{{Name: "T", Value: "ciopConfigContent"}},
	}, {
		description:   "bad CM key is handled",
		sourceEnv:     []v1.EnvVar{{Name: "T", ValueFrom: makeCMReference(ciOperatorConfigsCMName, "filename")}},
		configs:       &fakeCiopConfig{fakeFiles: map[string]string{}},
		expectedError: true,
	},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			job := makeTestingPresubmit(tc.sourceEnv)
			expectedJob := makeTestingPresubmit(tc.expectedEnv)

			newJob, err := inlineCiOpConfig(job, testTargetRepo, tc.configs, testLogger)

			if tc.expectedError && err == nil {
				t.Errorf("Expected inlineCiopConfig() to return an error, none returned")
				return
			}

			if !tc.expectedError {
				if err != nil {
					t.Errorf("Unexpected error returned by inlineCiOpConfig(): %v", err)
					return
				}

				if !equality.Semantic.DeepEqual(expectedJob, newJob) {
					t.Errorf("Returned job differs from expected:\n%s", diff.ObjectDiff(expectedJob, newJob))
				}
			}
		})
	}
}

func TestMakeRehearsalPresubmit(t *testing.T) {
	testCases := []struct {
		source   *config.Presubmit
		pr       int
		expected *config.Presubmit
	}{{
		source: &config.Presubmit{
			JobBase: config.JobBase{
				Name: "pull-ci-openshift-ci-operator-master-build",
				Spec: &v1.PodSpec{
					Containers: []v1.Container{{
						Command: []string{"ci-operator"},
						Args:    []string{"arg", "arg", "arg"},
					}},
				},
			},
			Context:  "ci/prow/build",
			Brancher: config.Brancher{Branches: []string{"^master$"}},
		},
		pr: 123,
		expected: &config.Presubmit{
			JobBase: config.JobBase{
				Name: "rehearse-123-pull-ci-openshift-ci-operator-master-build",
				Spec: &v1.PodSpec{
					Containers: []v1.Container{{
						Command: []string{"ci-operator"},
						Args:    []string{"arg", "arg", "arg", "--git-ref=openshift/ci-operator@master"},
					}},
				},
			},
			Context:  "ci/rehearse/openshift/ci-operator/build",
			Brancher: config.Brancher{Branches: []string{"^master$"}},
		},
	}}
	for _, tc := range testCases {
		rehearsal, err := makeRehearsalPresubmit(tc.source, "openshift/ci-operator", tc.pr)
		if err != nil {
			t.Errorf("Unexpected error in makeRehearsalPresubmit: %v", err)
		}
		if !equality.Semantic.DeepEqual(tc.expected, rehearsal) {
			t.Errorf("Expected rehearsal Presubmit differs:\n%s", diff.ObjectDiff(tc.expected, rehearsal))
		}
	}
}
