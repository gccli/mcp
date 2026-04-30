package ssh

import "fmt"

// Options SSH 连接参数
type Options struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

// ValidateOptions 验证 SSH 连接参数
func ValidateOptions(opts Options) error {
	if opts.Host == "" {
		return fmt.Errorf("host 参数不能为空")
	}
	if opts.Password == "" && opts.PrivateKey == "" {
		return fmt.Errorf("必须提供 password 或 private_key 参数进行认证")
	}
	return nil
}

// DefaultUsername 返回默认用户名
func DefaultUsername() string {
	return "root"
}
