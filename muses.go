package muses

import (
	"bytes"
	"errors"
	"fmt"
	ogin "github.com/gin-gonic/gin"
	"github.com/i2eco/muses/pkg/app"
	"github.com/i2eco/muses/pkg/cmd"
	"github.com/i2eco/muses/pkg/common"
	"github.com/i2eco/muses/pkg/logger"
	"github.com/i2eco/muses/pkg/prom"
	"github.com/i2eco/muses/pkg/server/gin"
	"github.com/i2eco/muses/pkg/system"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"syscall"
)

type Muses struct {
	cfgByte       []byte
	callers       []common.Caller
	isSetConfig   bool
	filePath      string
	preRun        []common.PreRunFunc
	postRun       []common.PostRunFunc
	ext           string
	err           error
	router        func() *ogin.Engine
	isCmdRegister bool
	isGinRegister bool
}

var (
	MUSES_DEBUG = false
)

// 初始化环境变量，方便调试
func init() {
	if os.Getenv("MUSES_DEBUG") == "true" {
		MUSES_DEBUG = true
	}
}

// 注册相应组件
func Container(callerFuncs ...common.CallerFunc) (muses *Muses) {
	allCallers := []common.CallerFunc{app.Register, logger.Register, prom.Register}
	allCallers = append(allCallers, callerFuncs...)
	muses = &Muses{}
	callers, err := sortCallers(allCallers)
	if err != nil {
		muses.err = err
		return
	}
	muses.callers = callers
	// todo 后面变成map，判断是否存在这个组件
	for _, caller := range muses.callers {
		name := getCallerName(caller)
		// 说明启动cmd配置，那么就不需要在setConfig
		if name == common.ModCmdName {
			muses.isCmdRegister = true
		}
		if name == common.ModGinName {
			muses.isGinRegister = true
		}
	}
	// 初始化 启动信息
	system.InitRunInfo()
	return
}

// 设置gin路由
func (m *Muses) SetGinRouter(router func() *ogin.Engine) *Muses {
	if m.err != nil {
		return m
	}
	m.router = router
	return m
}

// 在container之前运行
func (m *Muses) SetPreRun(f ...common.PreRunFunc) *Muses {
	m.preRun = f
	return m
}

// 在container之后运行
func (m *Muses) SetPostRun(f ...common.PostRunFunc) *Muses {
	m.postRun = f
	return m
}

func (m *Muses) SetRootCommand(f func(cobraCommand *cobra.Command)) {
	f(cmd.GetRootCmd())
}

func (m *Muses) SetStartCommand(f func(cobraCommand *cobra.Command)) {
	f(cmd.InitStartCommand(m.startFn))
}

// 设置配置
func (m *Muses) SetCfg(cfg interface{}) *Muses {
	if m.err != nil {
		return m
	}
	var err error
	var cfgByte []byte
	switch cfg.(type) {
	case string:
		m.filePath = cmd.ConfigPath
		err = isPathExist(m.filePath)
		if err != nil {
			m.err = err
			return m
		}

		ext := filepath.Ext(m.filePath)

		if len(ext) <= 1 {
			m.err = errors.New("config file ext is error")
			return m
		}
		m.ext = ext[1:]

		cfgByte, err = parseFile(cfg.(string))
		if err != nil {
			m.err = err
			return m
		}
		m.filePath = cfg.(string)
	case []byte:
		cfgByte = cfg.([]byte)
	default:
		m.err = fmt.Errorf("type is error %s", cfg)
		return m
	}
	m.cfgByte = cfgByte
	m.isSetConfig = true
	m.initViper()
	return m
}

func (m *Muses) Run() (err error) {
	if m.err != nil {
		err = m.err
		return
	}

	if m.isCmdRegister {
		cmd.InitStartCommand(m.startFn)
		cmd.AddStartCommand()
		if err := cmd.Execute(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		if !m.isSetConfig {
			err = errors.New("config is not exist")
			return
		}
		err = m.mustRun()
	}
	return
}

func (m *Muses) initViper() {
	rBytes := bytes.NewReader(m.cfgByte)
	viper.SetConfigType(m.ext)
	viper.AutomaticEnv() // read in environment variables that match
	err := viper.ReadConfig(rBytes)
	if err != nil {
		m.printInfo("Using config file:", viper.ConfigFileUsed())
	}
	//viper.Debug()
}

func isPathExist(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	return err
}

// 回调start函数
func (m *Muses) startFn(cobraCommand *cobra.Command, args []string) (err error) {
	fmt.Println(system.BuildInfo.LongForm())

	if m.isCmdRegister {
		m.SetCfg(cmd.ConfigPath)
		if m.err != nil {
			err = m.err
			return
		}
	}

	err = m.mustRun()
	if err != nil {
		return
	}

	if m.isGinRegister {
		addr := gin.Config().Muses.Server.Gin.Addr
		// 如果存在命令行的addr，覆盖配置里的addr
		if cmd.Addr != "" {
			addr = cmd.Addr
		}

		// 主服务器
		
		if err := m.router().Run(addr); err != nil {
			logger.DefaultLogger().Error("Server err", zap.String("err", err.Error()))
		}
	}
	return
}

// 每个项目都必须执行的run
func (m *Muses) mustRun() (err error) {
	// 运行前置指令
	for _, f := range m.preRun {
		err = f()
		if err != nil {
			return
		}
	}
	for _, caller := range m.callers {
		name := getCallerName(caller)
		m.printInfo("module", name, "cfg start")
		if err = caller.InitCfg(m.cfgByte); err != nil {
			m.printInfo("module", name, "init config error")
			return
		}
		m.printInfo("module", name, "cfg end")
		m.printInfo("module", name, "init caller start")
		if err = caller.InitCaller(); err != nil {
			m.printInfo("module", name, "init caller error")
			return
		}
		m.printInfo("module", name, "init caller ok")
	}

	// 运行后置指令
	for _, f := range m.postRun {
		err = f()
		if err != nil {
			return
		}
	}

	return
}

// todo 高亮
func (m *Muses) printInfo(info ...interface{}) {
	if MUSES_DEBUG {
		fmt.Println(info)
	}
}
