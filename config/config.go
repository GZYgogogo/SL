package config

import (
	"encoding/json"
	"os"
)

// Config 定义所有信誉计算参数，可从 JSON 文件加载
// 论文算法参数:
// Gamma: 不确定性对信誉的影响系数 (默认0.5)
// Rho1, Rho2: 整合权重 (交互频率、轨迹相似度), Rho1+Rho2=1
// Zeta, Sigma: 时效性权重 (近期、过去), Zeta+Sigma=1, 推荐Zeta=0.7
// Theta, Tau: 正负事件时效性衰减因子
// Psi1, Psi2, Psi3: 轨迹相似度权重 (速度、位置、方向), Psi1+Psi2+Psi3=1
// TRecent: 近期事件时间阈值 (秒)

type Config struct {
	Gamma   float64 `json:"gamma"`
	Rho1    float64 `json:"rho1"`
	Rho2    float64 `json:"rho2"`
	Zeta    float64 `json:"zeta"`
	Sigma   float64 `json:"sigma"`
	Theta   float64 `json:"theta"`
	Tau     float64 `json:"tau"`
	Psi1    float64 `json:"psi1"`
	Psi2    float64 `json:"psi2"`
	Psi3    float64 `json:"psi3"`
	TRecent float64 `json:"t_recent"`
}

// LoadConfig 从指定路径加载 JSON 配置
func LoadConfig(path string) (Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
