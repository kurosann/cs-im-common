package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"vane-common/pkg/logger"
	"vane-common/server"
)

type ServerCmd struct {
	ctx        context.Context
	rootCmd    *cobra.Command
	mainServer *server.MainServer
	cmds       []*cobra.Command

	reload func() error
	init   func(configPath string) error

	configPath  string
	versionInfo *VersionConfig
}

type VersionConfig struct {
	Version   string
	Branch    string
	Revision  string
	BuildTime string
}

func NewServerCmd(mainServer *server.MainServer) *ServerCmd {
	return &ServerCmd{
		mainServer: mainServer,
	}
}

func (s *ServerCmd) WithReload(reload func() error) *ServerCmd {
	s.reload = reload
	return s
}

func (s *ServerCmd) WithContext(ctx context.Context) *ServerCmd {
	s.ctx = ctx
	return s
}

func (s *ServerCmd) Reload() error {
	return s.reload()
}

func (s *ServerCmd) WithInit(init func(configPath string) error) *ServerCmd {
	s.init = init
	return s
}

func (s *ServerCmd) WithVersionInfo(versionInfo *VersionConfig) *ServerCmd {
	s.versionInfo = versionInfo
	return s
}

func (s *ServerCmd) Build() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start the server",
		Run: func(cmd *cobra.Command, args []string) {
			if err := s.init(s.configPath); err != nil {
				logger.Error(cmd.Context(), "init server error", zap.Error(err))
				os.Exit(1)
			}
			if err := s.mainServer.Init(cmd.Context()); err != nil {
				logger.Error(cmd.Context(), "init server error", zap.Error(err))
				os.Exit(1)
			}
			err := s.mainServer.Start(cmd.Context())
			if err != nil {
				logger.Error(cmd.Context(), "start server error", zap.Error(err))
				os.Exit(1)
			}
			s.mainServer.Wait(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&s.configPath, "config", "c", "", "config file path")
	return cmd
}

func (s *ServerCmd) BuildVersionCmd() *cobra.Command {
	if s.versionInfo == nil {
		panic("versionInfo is nil")
	}
	isJson := false
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show the version",
		Run: func(cmd *cobra.Command, args []string) {
			if isJson {
				bs, err := json.Marshal(s.versionInfo)
				if err != nil {
					cmd.Println(err)
					os.Exit(1)
				}
				fmt.Println(string(bs))
			} else {
				fmt.Printf("Version: %s\n", s.versionInfo.Version)
				fmt.Printf("Branch: %s\n", s.versionInfo.Branch)
				fmt.Printf("Revision: %s\n", s.versionInfo.Revision)
				fmt.Printf("BuildTime: %s\n", s.versionInfo.BuildTime)
			}
		},
	}
	cmd.Flags().BoolVarP(&isJson, "json", "j", false, "show the version in json format")
	return cmd
}
func (s *ServerCmd) AddCmd(cmd *cobra.Command) *ServerCmd {
	s.cmds = append(s.cmds, cmd)
	return s
}
func (s *ServerCmd) Start() {
	if s.rootCmd == nil {
		s.rootCmd = &cobra.Command{}
	}
	if s.ctx != nil {
		s.rootCmd.SetContext(s.ctx)
	}
	s.rootCmd.AddCommand(s.Build())
	s.rootCmd.AddCommand(s.BuildVersionCmd())
	for _, cmd := range s.cmds {
		s.rootCmd.AddCommand(cmd)
	}
	if err := s.rootCmd.Execute(); err != nil {
		logger.Error(s.ctx, "start server error", zap.Error(err))
		os.Exit(1)
	}
}
