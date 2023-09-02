package main

import (
	"reflect"
	"testing"
)

func Test_parseMetadata(t *testing.T) {
	type args struct {
		src []byte
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{"happy path",
			args{[]byte("title: Test Data Basic\npath: /basic\ndraft: false")},
			map[string]string{"title": "Test Data Basic", "draft": "false", "path": "/basic"},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMetadata(tt.args.src)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
