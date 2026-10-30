package activity

import (
	"os"

	"yatori-go-console/config"

	"gopkg.in/yaml.v3"
)

// getConfigFilePath 配置文件路径，支持 CONFIG_FILE_PATH 覆盖，便于 Docker 部署
func getConfigFilePath() string {
	path := os.Getenv("CONFIG_FILE_PATH")
	if path != "" {
		return path
	}
	return "./config.yaml"
}

// readSetting 读取配置文件中的 setting（AI、题库、邮件等全局配置仍由 config.yaml 管理）
func readSetting() config.Setting {
	data, err := os.ReadFile(getConfigFilePath())
	if err != nil {
		return config.Setting{}
	}
	type yamlConfig struct {
		Setting config.Setting `yaml:"setting"`
	}
	var cfg yamlConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return config.Setting{}
	}
	return cfg.Setting
}
