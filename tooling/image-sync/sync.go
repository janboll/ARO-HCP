package main

import (
	"context"

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

func DoSync() {
	cfg := NewSyncConfig()
	Log().Infow("Syncing images", "images", cfg.Images, "numberoftags", cfg.NumberOfTags)
	ctx := context.Background()
	qr := NewQuayRegistry(cfg)
	acr := NewAzureContainerRegistry(cfg)

	for _, image := range cfg.Images {
		tags, err := qr.GetTags(ctx, image)

		if err != nil {
			Log().Fatalw("Error getting tags", "error", err)
		}
		Log().Infow("Got tags from quay", "tags", tags)

		exists, err := acr.RepositoryExists(ctx, image)
		if err != nil {
			Log().Fatalw("Error getting repository information", "error", err)
		}

		if exists {
			acr_tags, err := acr.GetTags(ctx, image)
			if err != nil {
				Log().Fatalw("Error getting tags", "error", err)
			}

			Log().Infow("Got tags from acr", "tags", acr_tags)
		} else {
			Log().Infow("Repository does not exist", "repository", image)
		}
	}

}
