package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	prowconfig "k8s.io/test-infra/prow/config"

	jc "github.com/openshift/ci-operator-prowgen/pkg/jobconfig"
)

type options struct {
	prowJobConfigDir string

	help bool
}

func bindOptions(flag *flag.FlagSet) *options {
	opt := &options{}

	flag.StringVar(&opt.prowJobConfigDir, "prow-jobs-dir", "", "Path to a root of directory structure with Prow job config files (ci-operator/jobs in openshift/release)")
	flag.BoolVar(&opt.help, "h", false, "Show help for ci-operator-prowgen")

	return opt
}

func getProwJobConfig(path string) (string, *prowconfig.JobConfig, error) {
	directory := filepath.Dir(path)
	repo := filepath.Base(directory)
	organization := filepath.Base(filepath.Dir(directory))

	config, err := jc.ReadFromFile(path)
	if err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%s/%s", organization, repo), config, nil
}

func isCiOperatorPresubmit(job prowconfig.Presubmit) bool {
	return (job.Agent == "kubernetes" && job.Spec.Containers[0].Command[0] == "ci-operator")
}

func isGenericKubePresubmit(job prowconfig.Presubmit) bool {
	return (job.Agent == "kubernetes")
}

func sanitizeCommand(command, args []string) string {
	sane := fmt.Sprintf("%s %s", strings.Join(command, " "), strings.Join(args, " "))
	sane = strings.Replace(sane, "\\\n", " ", -1)
	sane = strings.Replace(sane, "\n", " ", -1)

	return sane

}

func describeKubePresubmit(job prowconfig.Presubmit) string {
	if len(job.Spec.Containers) == 1 {
		command := sanitizeCommand(job.Spec.Containers[0].Command, job.Spec.Containers[0].Args)
		return fmt.Sprintf("Presubmit '%s' based on '%s' running '%s' on branch(es) %s", job.Context, job.Spec.Containers[0].Image, command, strings.Join(job.Branches, ", "))
	}

	return fmt.Sprintf("Presubmit '%s' with %d containers running on branch(es) %s", job.Context, len(job.Spec.Containers), strings.Join(job.Branches, ", "))

}

func isCiOperatorPostsubmit(job prowconfig.Postsubmit) bool {
	return (job.Agent == "kubernetes" && job.Spec.Containers[0].Command[0] == "ci-operator")
}

func describePresubmit(job prowconfig.Presubmit) string {
	return fmt.Sprintf("Presubmit '%s' running '%s %s' on branch(es) %s", job.Context, strings.Join(job.Spec.Containers[0].Command, " "), strings.Join(job.Spec.Containers[0].Args, " "), strings.Join(job.Branches, ", "))

}

func describePostsubmit(job prowconfig.Postsubmit) string {
	return fmt.Sprintf("Postsubmit '%s' running '%s %s' on branch(es) %s", job.Name, strings.Join(job.Spec.Containers[0].Command, " "), strings.Join(job.Spec.Containers[0].Args, " "), strings.Join(job.Branches, ", "))

}

func isJenkinsPresubmit(job prowconfig.Presubmit) bool {
	return job.Agent == "jenkins"
}

func isJenkinsPostsubmit(job prowconfig.Postsubmit) bool {
	return job.Agent == "jenkins"
}

func describe(configs map[string]*prowconfig.JobConfig) string {
	builder := strings.Builder{}

	for repo, jobs := range configs {
		builder.WriteString(fmt.Sprintf("Prow jobs for `%s`:\n", repo))
		jenkinsPresubmits := 0
		jenkinsPostsubmits := 0
		if jobs.Presubmits != nil {
			for _, job := range jobs.Presubmits[repo] {
				if isCiOperatorPresubmit(job) {
					builder.WriteString(fmt.Sprintf(" - %s\n", describePresubmit(job)))
				} else if isJenkinsPresubmit(job) {
					jenkinsPresubmits++
				} else if isGenericKubePresubmit(job) {
					builder.WriteString(fmt.Sprintf(" - %s\n", describeKubePresubmit(job)))
				} else {
					builder.WriteString(fmt.Sprintf(" - WARNING: %s\n", job.Name))
				}
			}
			if jenkinsPresubmits > 0 {
				builder.WriteString(fmt.Sprintf(" - %d presubmits backed by Jenkins\n", jenkinsPresubmits))
			}
		}
		if jobs.Postsubmits != nil {
			for _, job := range jobs.Postsubmits[repo] {
				if isCiOperatorPostsubmit(job) {
					builder.WriteString(fmt.Sprintf(" - %s\n", describePostsubmit(job)))
				} else if isJenkinsPostsubmit(job) {
					jenkinsPostsubmits++
				} else {
					builder.WriteString(fmt.Sprintf(" - WARNING: %s\n", job.Name))
				}
			}
			if jenkinsPostsubmits > 0 {
				builder.WriteString(fmt.Sprintf(" - %d postsubmits backed by Jenkins\n", jenkinsPostsubmits))
			}
		}
	}

	return builder.String()
}

func describeProwJobs(prowJobConfigDir string) (string, error) {
	configs := map[string]*prowconfig.JobConfig{}

	if err := filepath.Walk(prowJobConfigDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error encountered while walking Prow jobs: %v\n", err)
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".yaml" {
			repo, config, err := getProwJobConfig(path)
			if err != nil {
				return err
			}

			if _, hasKey := configs[repo]; !hasKey {
				configs[repo] = config
			} else {
				jc.Merge(configs[repo], config)
			}
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to process Prow jobs (%v)", err)
	}

	return describe(configs), nil
}

func main() {
	flagSet := flag.NewFlagSet("", flag.ExitOnError)
	opt := bindOptions(flagSet)
	flagSet.Parse(os.Args[1:])

	if opt.help {
		flagSet.Usage()
		os.Exit(0)
	}

	if len(opt.prowJobConfigDir) > 0 {
		description, err := describeProwJobs(opt.prowJobConfigDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "describe failed (%v)\n", err)
		}
		fmt.Printf(description)
	} else {
		fmt.Fprintf(os.Stderr, "describe tool needs the --prow-jobs-dir\n")
		os.Exit(1)
	}
}
