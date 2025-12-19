package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"proofread-exhibition/config"
	"proofread-exhibition/pkg/server"
	"proofread-exhibition/pkg/signals"
	"proofread-exhibition/pkg/util"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewRootCommand() *cobra.Command {
	var configFilePath string
	cmd := &cobra.Command{
		Use:   "",
		Short: "",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd:   true,
			DisableNoDescFlag:   true,
			DisableDescriptions: true,
			HiddenDefaultCmd:    true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.TryLoadFromDisk(configFilePath)
			if err != nil {
				zap.S().Errorf("读取本地配置文件错误:%s", err.Error())
				return
			}
			if errs := cfg.Validate(); len(errs) > 0 {
				zap.S().Errorf("本地配置文件验证错误:%s", errors.Join(errs...))
				return
			}
			ctx := signals.SetupSignalHandler()
			if err := run(cfg, ctx); err != nil {
				zap.S().Errorf("运行时错误:%s", err.Error())
				return
			}
		},
		Version: util.GetVersion().Version,
	}
	cmd.Flags().StringVarP(&configFilePath, "config", "c", "./etc/config.yaml", "配置文件路径")
	_ = cmd.MarkFlagRequired("config")
	_ = viper.BindPFlag("config", cmd.Flags().Lookup("config"))
	return cmd
}

func run(cfg *config.GlobalConfig, ctx context.Context) error {
	s := server.NewServer(cfg)
	g, c := errgroup.WithContext(ctx)
	g.Go(func() error {
		// 等待服务器启动后打开浏览器
		go func() {
			time.Sleep(500 * time.Millisecond) // 等待服务器启动
			openBrowser(fmt.Sprintf("http://localhost:%d", cfg.Port))
		}()
		return s.Run()
	})
	zap.S().Debugf("http server[:%d] 已经运行...", cfg.Port)
	g.Go(func() error {
		<-c.Done()
		s.GracefulShutdown(ctx)
		return nil
	})
	return g.Wait()

}

// openBrowser 打开浏览器
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		zap.S().Warnf("不支持的操作系统: %s", runtime.GOOS)
		return
	}

	if err := cmd.Start(); err != nil {
		zap.S().Warnf("打开浏览器失败: %s", err.Error())
	}
}
