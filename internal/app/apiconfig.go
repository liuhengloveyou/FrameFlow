package app

// APIModuleConfig 对应 YAML 根键 `api:`（由 app.Init 内 LoadConfig 填充，供 api.Init 只读使用）。
type APIModuleConfig struct {
	Listen     string   `yaml:"listen"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	LocalAuth  bool     `yaml:"local_auth"`
	BasePath   string   `yaml:"base_path"`
	StaticDir  string   `yaml:"static_dir"`
	Origin     string   `yaml:"origin"`
	TLSListen  string   `yaml:"tls_listen"`
	TLSCert    string   `yaml:"tls_cert"`
	TLSKey     string   `yaml:"tls_key"`
	UnixListen string   `yaml:"unix_listen"`
	AllowPaths []string `yaml:"allow_paths"`
}

// APICfg 为合并配置后的 api 段快照；须先 app.Init 再调用 api.Init。
var APICfg APIModuleConfig
