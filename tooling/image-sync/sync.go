package main

import (
	"context"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker"
	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	"github.com/spf13/viper"
)

type SyncConfig struct {
	Images         []string
	NumberOfTags   int
	QuaySecretFile string
	AcrRegistry    string
	RequestTimeout int
	SkipLatest     bool
}

func NewSyncConfig() *SyncConfig {
	var sc *SyncConfig
	v := viper.GetViper()
	v.SetDefault("numberoftags", 10)
	v.SetDefault("requesttimeout", 10)
	v.SetDefault("skiplatest", true)

	if err := v.Unmarshal(&sc); err != nil {
		Log().Fatalw("Error while unmarshalling configuration %s", err.Error())
	}
	Log().Debugw("Using configuration", "config", sc)
	return sc
}

func Copy(ctx context.Context, dstreference, srcreference string, dstauth, srcauth *types.DockerAuthConfig) error {
	policyctx, err := signature.NewPolicyContext(&signature.Policy{
		Default: signature.PolicyRequirements{
			signature.NewPRInsecureAcceptAnything(),
		},
	})
	if err != nil {
		return err
	}

	src, err := docker.ParseReference("//" + srcreference)
	if err != nil {
		return err
	}

	dst, err := docker.ParseReference("//" + dstreference)
	if err != nil {
		return err
	}

	_, err = copy.Image(ctx, policyctx, dst, src, &copy.Options{
		SourceCtx: &types.SystemContext{
			DockerAuthConfig: srcauth,
		},
		DestinationCtx: &types.SystemContext{
			DockerAuthConfig: dstauth,
		},
		// Images that we mirror shouldn't change, so we can use the
		// optimisation that checks if the source and destination manifests are
		// equal before attempting to push it (and sending no blobs because
		// they're all already there)
		// OptimizeDestinationImageAlreadyExists: true,
	})

	return err
}

func DoSync() {
	cfg := NewSyncConfig()
	Log().Infow("Syncing images", "images", cfg.Images, "numberoftags", cfg.NumberOfTags)
	ctx := context.Background()
	qr := NewQuayRegistry(cfg, "")
	acr := NewAzureContainerRegistry(cfg)

	t, err := acr.GetPullSecret(ctx)

	if err != nil {
		Log().Fatalw("Error getting pull secret", "error", err)
	}

	for _, image := range cfg.Images {
		quayTags, err := qr.GetTags(ctx, image)

		if err != nil {
			Log().Fatalw("Error getting tags", "error", err)
		}
		Log().Infow("Got tags from quay", "tags", quayTags)

		exists, err := acr.RepositoryExists(ctx, image)
		if err != nil {
			Log().Fatalw("Error getting repository information", "error", err)
		}

		if exists {
			acrTags, err := acr.GetTags(ctx, image)
			if err != nil {
				Log().Fatalw("Error getting tags", "error", err)
			}

			Log().Infow("Got tags from acr", "tags", acrTags)
		} else {
			Log().Infow("Repository does not exist", "repository", image)
		}

	}

	acrAuth := types.DockerAuthConfig{Username: "00000000-0000-0000-0000-000000000000", Password: t.RefreshToken}
	quayAuth := types.DockerAuthConfig{Username: "jboll", Password: ""}

	err = Copy(ctx, "devarohcp.azurecr.io/jboll/testing:abcdef", "quay.io/jboll/testing:abcdef", &acrAuth, &quayAuth)
	if err != nil {
		Log().Fatalw("Error copying image", "error", err.Error())

	}
}
