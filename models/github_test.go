package models

import "testing"

func Test_cleanBotUserName(t *testing.T) {
	type args struct {
		username string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Main Test Case",
			args: args{
				username: "infratest[bot]",
			},
			want: "infratest",
		},
		{
			name: "Without suffix bot",
			args: args{
				username: "infratest",
			},
			want: "infratest",
		},
		{
			name: "Bot only",
			args: args{
				username: "[bot]",
			},
			want: "",
		},
		{
			name: "Bot at start",
			args: args{
				username: "[bot]infratest",
			},
			want: "[bot]infratest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanBotUserName(tt.args.username); got != tt.want {
				t.Errorf("cleanBotUserName() = %v, want %v", got, tt.want)
			}
		})
	}
}
