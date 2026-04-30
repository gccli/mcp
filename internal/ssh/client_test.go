package ssh

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	cache := NewConnectionCache()

	tests := []struct {
		name        string
		opts        Options
		wantHost    string
		wantUser    string
		wantPass    string
		wantPrivKey string
	}{
		{
			name:        "密码认证",
			opts:        Options{Host: "192.168.1.1", Username: "root", Password: "test123"},
			wantHost:    "192.168.1.1",
			wantUser:    "root",
			wantPass:    "test123",
			wantPrivKey: "",
		},
		{
			name:        "私钥认证",
			opts:        Options{Host: "192.168.1.1", Username: "root", PrivateKey: "/path/to/key"},
			wantHost:    "192.168.1.1",
			wantUser:    "root",
			wantPass:    "",
			wantPrivKey: "/path/to/key",
		},
		{
			name:        "默认用户名为root",
			opts:        Options{Host: "192.168.1.1", Password: "test123"},
			wantHost:    "192.168.1.1",
			wantUser:    "root",
			wantPass:    "test123",
			wantPrivKey: "",
		},
		{
			name:        "指定其他用户名",
			opts:        Options{Host: "192.168.1.1", Username: "admin", Password: "test123"},
			wantHost:    "192.168.1.1",
			wantUser:    "admin",
			wantPass:    "test123",
			wantPrivKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.opts, cache)
			if client == nil {
				t.Fatal("NewClient 返回 nil")
			}
			if client.opts.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", client.opts.Host, tt.wantHost)
			}
			if client.opts.Username != tt.wantUser {
				t.Errorf("Username = %q, want %q", client.opts.Username, tt.wantUser)
			}
			if client.opts.Password != tt.wantPass {
				t.Errorf("Password = %q, want %q", client.opts.Password, tt.wantPass)
			}
			if client.opts.PrivateKey != tt.wantPrivKey {
				t.Errorf("PrivateKey = %q, want %q", client.opts.PrivateKey, tt.wantPrivKey)
			}
		})
	}
}

func TestBuildSudoCommand(t *testing.T) {
	cache := NewConnectionCache()
	client := NewClient(Options{Host: "test", Username: "root", Password: "test"}, cache)

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "简单命令",
			command:  "ls -la",
			expected: "sudo ls -la",
		},
		{
			name:     "带管道的命令",
			command:  "cat /var/log/syslog | grep error",
			expected: "sudo cat /var/log/syslog | grep error",
		},
		{
			name:     "单个命令",
			command:  "id",
			expected: "sudo id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildSudoCommand(tt.command)
			if result != tt.expected {
				t.Errorf("buildSudoCommand(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name:    "缺少host",
			opts:    Options{Username: "root", Password: "test"},
			wantErr: true,
		},
		{
			name:    "缺少认证方式",
			opts:    Options{Host: "192.168.1.1", Username: "root"},
			wantErr: true,
		},
		{
			name:    "密码认证有效",
			opts:    Options{Host: "192.168.1.1", Username: "root", Password: "test"},
			wantErr: false,
		},
		{
			name:    "私钥认证有效",
			opts:    Options{Host: "192.168.1.1", Username: "root", PrivateKey: "/path/to/key"},
			wantErr: false,
		},
		{
			name:    "密码和私钥同时提供",
			opts:    Options{Host: "192.168.1.1", Username: "root", Password: "test", PrivateKey: "/path/to/key"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptions(tt.opts)
			if tt.wantErr && err == nil {
				t.Errorf("期望返回错误，但得到 nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("期望无错误，但得到 %v", err)
			}
		})
	}
}
