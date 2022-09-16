package slice

import "testing"

func TestContains(t *testing.T) {
	type args struct {
		list interface{}
		elem interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "[Success] string slice has 'metallica'",
			args: args{
				list: []string{"a", "bb", "metallica", "abc"},
				elem: "metallica",
			},
			want: true,
		},
		{
			name: "[Success] string slice does not have 'metallica'",
			args: args{
				list: []string{"a", "bbb", "abc"},
				elem: "metallica",
			},
			want: false,
		},
		{
			name: "[Success] Integer slice has 100",
			args: args{
				list: []int64{1, 3, 100, 21},
				elem: 100,
			},
			want: true,
		},
		{
			name: "[Success] Integer slice  does not have 100",
			args: args{
				list: []int64{1, 3, 21},
				elem: 100,
			},
			want: false,
		},
		{
			name: "[Error] If the slice and element types are different",
			args: args{
				list: []string{"a", "bb", "metallica", "abc"},
				elem: -100,
			},
			want: false,
		},
		{
			name: "[Error] If both arguments are slices",
			args: args{
				list: []string{"a", "bb", "metallica", "abc"},
				elem: []int64{1, 3, 21},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.list, tt.args.elem); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}
