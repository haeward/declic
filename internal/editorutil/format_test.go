package editorutil

import "testing"

func TestFormatCode(t *testing.T) {
	t.Run("go uses gofmt", func(t *testing.T) {
		got := FormatCode("golang", "package main\nfunc twoSum(a int,b int)int{\nreturn a+b\n}", 4)
		want := "package main\n\nfunc twoSum(a int, b int) int {\n\treturn a + b\n}\n"
		if got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("brace language re-indents respecting strings and comments", func(t *testing.T) {
		src := "class Solution {\npublic int f() {\nString s = \"{\"; // }\nif (true) {\nreturn 1;\n}\n}\n}"
		got := FormatCode("java", src, 2)
		want := "class Solution {\n  public int f() {\n    String s = \"{\"; // }\n    if (true) {\n      return 1;\n    }\n  }\n}\n"
		if got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("python preserves indentation and expands tabs", func(t *testing.T) {
		src := "def solve(x):   \r\n\tif x > 0:\t\r\n\t\treturn x\r\n\r\n\r\n\r\n\treturn 0  \r\n"
		got := FormatCode("python3", src, 4)
		want := "def solve(x):\n    if x > 0:\n        return x\n\n    return 0\n"
		if got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}
