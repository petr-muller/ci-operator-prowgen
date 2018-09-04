package jobconfig

import (
	prowconfig "k8s.io/test-infra/prow/config"
)

// Merge jobs from the `source` one to to `destination` one. Jobs are matched
// by name. All jobs from `source` will be present in `destination` - if there
// were jobs with the same name in `destination`, they will be overwritten. All
// jobs in `destination` that are not overwritten this way stay untouched.
func Merge(destination, source *prowconfig.JobConfig) {
	// We do the same thing for both Presubmits and Postsubmits
	if source.Presubmits != nil {
		if destination.Presubmits == nil {
			destination.Presubmits = map[string][]prowconfig.Presubmit{}
		}
		for repo, jobs := range source.Presubmits {
			oldPresubmits, _ := destination.Presubmits[repo]
			destination.Presubmits[repo] = []prowconfig.Presubmit{}
			newJobs := map[string]prowconfig.Presubmit{}
			for _, job := range jobs {
				newJobs[job.Name] = job
			}
			for _, newJob := range source.Presubmits[repo] {
				destination.Presubmits[repo] = append(destination.Presubmits[repo], newJob)
			}

			for _, oldJob := range oldPresubmits {
				if _, hasKey := newJobs[oldJob.Name]; !hasKey {
					destination.Presubmits[repo] = append(destination.Presubmits[repo], oldJob)
				}
			}
		}
	}
	if source.Postsubmits != nil {
		if destination.Postsubmits == nil {
			destination.Postsubmits = map[string][]prowconfig.Postsubmit{}
		}
		for repo, jobs := range source.Postsubmits {
			oldPostsubmits, _ := destination.Postsubmits[repo]
			destination.Postsubmits[repo] = []prowconfig.Postsubmit{}
			newJobs := map[string]prowconfig.Postsubmit{}
			for _, job := range jobs {
				newJobs[job.Name] = job
			}
			for _, newJob := range source.Postsubmits[repo] {
				destination.Postsubmits[repo] = append(destination.Postsubmits[repo], newJob)
			}

			for _, oldJob := range oldPostsubmits {
				if _, hasKey := newJobs[oldJob.Name]; !hasKey {
					destination.Postsubmits[repo] = append(destination.Postsubmits[repo], oldJob)
				}
			}
		}
	}
}
