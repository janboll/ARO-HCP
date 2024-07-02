package main

import "github.com/spf13/viper"

type SyncConfig struct {
	Images       []string
	Numberoftags int
}

func NewSyncConfig() *SyncConfig {
	var sc *SyncConfig
	v := viper.GetViper()
	v.SetDefault("numberoftags", 10)
	if err := v.Unmarshal(&sc); err != nil {
		Log().Fatalw("Error while unmarshalling configuration %s", err.Error())
	}
	return sc
}

func DoSync() {
	cfg := NewSyncConfig()
	Log().Infow("Syncing images", "images", cfg.Images, "numberoftags", cfg.Numberoftags)

}
