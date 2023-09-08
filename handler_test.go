package main

import (
	"reflect"
	"testing"
	"time"
)

func Test_parseMetadata_old(t *testing.T) {
	type args struct {
		src map[string]any
	}
	tests := []struct {
		name    string
		args    args
		want    metadata
		wantErr bool
	}{
		{"required fields",
			args{map[string]any{
				"title": "Test Data Basic",
				"path":  "/basic",
				"draft": false,
			}},
			metadata{Title: "Test Data Basic", Path: "/basic", Draft: false, Tags: []string{}, Date: time.Time{}},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMetadata(tt.args.src)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("got: %+v", got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
