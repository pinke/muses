package stat

import "github.com/pinke/muses/pkg/common"

type Cfg struct {
	Muses struct {
		Server struct {
			Stat CallerCfg `toml:"stat"`
		} `toml:"server"`
	} `toml:"muses"`
}

type CallerCfg struct {
	Addr         string
	ReadTimeout  common.Duration
	WriteTimeout common.Duration
}
