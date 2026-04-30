package ssh

import (
	"testing"
)

func TestExecToolParams(t *testing.T) {
	// 验证工具参数结构
	tests := []struct {
		name    string
		params  execToolParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "完整参数-密码认证",
			params: execToolParams{
				Host:     "192.168.1.1",
				Username: "root",
				Password: "test123",
				Command:  "ls -la",
			},
			wantErr: false,
		},
		{
			name: "完整参数-私钥认证",
			params: execToolParams{
				Host:       "192.168.1.1",
				Username:   "root",
				PrivateKey: "/path/to/key",
				Command:    "ls -la",
			},
			wantErr: false,
		},
		{
			name: "默认用户名",
			params: execToolParams{
				Host:     "192.168.1.1",
				Password: "test123",
				Command:  "ls -la",
			},
			wantErr: false,
		},
		{
			name: "缺少host",
			params: execToolParams{
				Username: "root",
				Password: "test123",
				Command:  "ls -la",
			},
			wantErr: true,
			errMsg:  "host",
		},
		{
			name: "缺少command",
			params: execToolParams{
				Host:     "192.168.1.1",
				Username: "root",
				Password: "test123",
			},
			wantErr: true,
			errMsg:  "command",
		},
		{
			name: "缺少认证方式",
			params: execToolParams{
				Host:     "192.168.1.1",
				Username: "root",
				Command:  "ls -la",
			},
			wantErr: true,
			errMsg:  "认证",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExecToolParams(tt.params)
			if tt.wantErr && err == nil {
				t.Errorf("期望返回错误，但得到 nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("期望无错误，但得到 %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() == "" {
					t.Errorf("错误消息为空，期望包含 %q", tt.errMsg)
				}
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Errorf("NewServer 返回错误: %v", err)
	}
	if server == nil {
		t.Error("NewServer 返回 nil")
	}
}

func TestToOptions(t *testing.T) {
	tests := []struct {
		name     string
		params   execToolParams
		wantHost string
		wantUser string
	}{
		{
			name: "指定用户名",
			params: execToolParams{
				Host:     "192.168.1.1",
				Username: "admin",
				Password: "test123",
				Command:  "ls",
			},
			wantHost: "192.168.1.1",
			wantUser: "admin",
		},
		{
			name: "默认用户名",
			params: execToolParams{
				Host:     "192.168.1.1",
				Password: "test123",
				Command:  "ls",
			},
			wantHost: "192.168.1.1",
			wantUser: "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := toOptions(tt.params)
			if opts.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", opts.Host, tt.wantHost)
			}
			if opts.Username != tt.wantUser {
				t.Errorf("Username = %q, want %q", opts.Username, tt.wantUser)
			}
		})
	}
}
