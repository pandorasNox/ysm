package main

import (
	"reflect"
	"testing"
)

func Test_removeByPath(t *testing.T) {
	type args struct {
		data    map[string]interface{}
		keyPath string
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "delete one element",
			args: args{
				data: map[string]interface{}{
					"a": "abc",
				},
				keyPath: "a",
			},
			want: map[string]interface{}{},
		},
		{
			name: "delete one nested element",
			args: args{
				data: map[string]interface{}{
					"a": map[string]interface{}{
						"b": "abc",
						"c": "keep me",
					},
				},
				keyPath: "a.b",
			},
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"c": "keep me",
				},
			},
		},
		{
			name: "delete missing element",
			args: args{
				data: map[string]interface{}{
					"a": map[string]interface{}{
						"c": "keep me",
					},
				},
				keyPath: "a.b",
			},
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"c": "keep me",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeByPath(tt.args.data, tt.args.keyPath)
			got := tt.args.data
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeByPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
