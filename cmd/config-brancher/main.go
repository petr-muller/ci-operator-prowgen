package main

import (
	"flag"
	"os"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"

	"github.com/openshift/ci-operator/pkg/api"

	"github.com/openshift/ci-operator-prowgen/pkg/config"
	"github.com/openshift/ci-operator-prowgen/pkg/promotion"
)

func gatherOptions() promotion.Options {
	o := promotion.Options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	o.Bind(fs)
	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("could not parse input")
	}
	return o
}

func main() {
	o := gatherOptions()
	if err := o.Validate(); err != nil {
		logrus.Fatalf("Invalid options: %v", err)
	}

	var toCommit []config.DataWithInfo
	if err := config.OperateOnCIOperatorConfigDir(o.ConfigDir, func(configuration *api.ReleaseBuildConfiguration, info *config.Info) error {
		if (o.Org != "" && o.Org != info.Org) || (o.Repo != "" && o.Repo != info.Repo) {
			return nil
		}
		for _, output := range generateBranchedConfigs(o.CurrentRelease, o.FutureRelease, config.DataWithInfo{Configuration: *configuration, Info: *info}) {
			if !o.Confirm {
				output.Logger().Info("Would commit new file.")
				continue
			}

			// we are walking the config so we need to commit once we're done
			toCommit = append(toCommit, output)
		}

		return nil
	}); err != nil {
		logrus.WithError(err).Fatal("Could not branch configurations.")
	}

	var failed bool
	for _, output := range toCommit {
		if err := output.CommitTo(o.ConfigDir); err != nil {
			failed = true
		}
	}
	if failed {
		logrus.Fatal("Failed to commit configuration to disk.")
	}
}

func generateBranchedConfigs(currentRelease, futureRelease string, input config.DataWithInfo) []config.DataWithInfo {
	if !(promotion.PromotesOfficialImages(&input.Configuration) && input.Configuration.PromotionConfiguration.Name == currentRelease) {
		return nil
	}
	input.Logger().Info("Branching configuration.")
	// we need a deep copy and this is a simple albeit expensive hack to get there
	raw, err := yaml.Marshal(input.Configuration)
	if err != nil {
		input.Logger().WithError(err).Error("failed to marshal input CI Operator configuration")
		return nil
	}
	var futureConfig api.ReleaseBuildConfiguration
	if err := yaml.Unmarshal(raw, &futureConfig); err != nil {
		input.Logger().WithError(err).Error("failed to unmarshal input CI Operator configuration")
		return nil
	}

	// in order to branch this, we need to update where we're promoting
	// to and from where we're building a release payload
	futureConfig.PromotionConfiguration.Name = futureRelease
	futureConfig.ReleaseTagConfiguration.Name = futureRelease

	futureBranchForCurrentPromotion, futureBranchForFuturePromotion, err := promotion.DetermineReleaseBranches(currentRelease, futureRelease, input.Info.Branch)
	if err != nil {
		input.Logger().WithError(err).Error("could not determine future branch that would promote to current imagestream")
		return nil
	}

	return []config.DataWithInfo{
		// this config keeps the current promotion but runs on a new branch
		{Configuration: input.Configuration, Info: copyInfoSwappingBranches(input.Info, futureBranchForCurrentPromotion)},
		// this config is the future promotion on the future branch
		{Configuration: futureConfig, Info: copyInfoSwappingBranches(input.Info, futureBranchForFuturePromotion)},
	}
}

func copyInfoSwappingBranches(input config.Info, newBranch string) config.Info {
	intermediate := &input
	output := *intermediate
	output.Branch = newBranch
	return output
}
