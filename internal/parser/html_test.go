package parser

import "testing"

func TestExtractTitle_TitleTag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"basic title", "<html><head><title>Hello World</title></head></html>", "Hello World"},
		{"title with whitespace", "<title>  Spaced Title  </title>", "Spaced Title"},
		{"empty title", "<title></title>", ""},
		{"no title", "<html><body><h1>No Title</h1></body></html>", ""},
		{"title with entities", "<title>Tom &amp; Jerry</title>", "Tom & Jerry"},
		{"title with unicode escape", `<title>Tokyo \u6771\u4eac</title>`, "Tokyo 東京"},
		{"title with numeric entity", "<title>Price &#36;100</title>", "Price $100"},
		{"title with hex entity", "<title>Less &#x3C; More</title>", "Less < More"},
		// Inside <title>, HTML tags are treated as raw text (RCDATA), not parsed as elements
		{"title with nested tags", "<title>My <b>Bold</b> Title</title>", "My <b>Bold</b> Title"},
		{"multiple title tags", "<title>First</title><title>Second</title>", "First"},
		{"title in body not head", "<html><body><title>Body Title</title></body></html>", "Body Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTitle(tt.body)
			if got != tt.want {
				t.Errorf("ExtractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTitle_OGTitle(t *testing.T) {
	body := `<html><head>
		<meta property="og:title" content="OG Title Value">
	</head></html>`
	got := ExtractTitle(body)
	if got != "OG Title Value" {
		t.Errorf("ExtractTitle() = %q, want %q", got, "OG Title Value")
	}
}

func TestExtractTitle_TwitterTitle(t *testing.T) {
	body := `<html><head>
		<meta name="twitter:title" content="Twitter Title Value">
	</head></html>`
	got := ExtractTitle(body)
	if got != "Twitter Title Value" {
		t.Errorf("ExtractTitle() = %q, want %q", got, "Twitter Title Value")
	}
}

func TestExtractTitle_Priority(t *testing.T) {
	body := `<html><head>
		<title>HTML Title</title>
		<meta property="og:title" content="OG Title">
		<meta name="twitter:title" content="Twitter Title">
	</head></html>`
	got := ExtractTitle(body)
	if got != "HTML Title" {
		t.Errorf("ExtractTitle() = %q, want %q (should prefer <title>)", got, "HTML Title")
	}
}

func TestExtractTitle_OGFallback(t *testing.T) {
	body := `<html><head>
		<meta property="og:title" content="OG Title">
		<meta name="twitter:title" content="Twitter Title">
	</head></html>`
	got := ExtractTitle(body)
	if got != "OG Title" {
		t.Errorf("ExtractTitle() = %q, want %q (should prefer og:title over twitter)", got, "OG Title")
	}
}

func TestExtractTitle_MalformedHTML(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"unclosed tags", "<html><head><title>Unclosed", "Unclosed"},
		{"empty string", "", ""},
		{"no HTML at all", "just plain text", ""},
		{"binary-like content", "\x00\x01\x02\x03", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTitle(tt.body)
			if got != tt.want {
				t.Errorf("ExtractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeTitleString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unicode escape", `\u0026`, "&"},
		{"html entity", "Tom &amp; Jerry", "Tom & Jerry"},
		{"numeric entity", "&#38;", "&"},
		{"hex entity", "&#x26;", "&"},
		{"multiple unicode escapes", `\u0048\u0065\u006C\u006C\u006F`, "Hello"},
		// \u0026 → &, then &amp; → & (double decode)
		{"mixed double decode", `\u0026amp;`, "&"},
		{"no escapes", "plain text", "plain text"},
		{"invalid unicode kept", `\uXXXX`, `\uXXXX`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeTitleString(tt.input)
			if got != tt.want {
				t.Errorf("decodeTitleString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountWordsAndLines(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantWords int
		wantLines int
	}{
		{"empty string", "", 0, 0},
		{"single word", "hello", 1, 1},
		{"multiple words single line", "hello world foo", 3, 1},
		{"multiple lines", "line one\nline two\nline three", 6, 3},
		{"trailing newline", "hello\n", 1, 2},
		{"only whitespace", "   \t  ", 0, 1},
		{"empty lines", "\n\n\n", 0, 4},
		{"mixed content", "word1 word2\n\nword3\n  word4  word5\n", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			words, lines := CountWordsAndLines(tt.text)
			if words != tt.wantWords || lines != tt.wantLines {
				t.Errorf("CountWordsAndLines() = (%d, %d), want (%d, %d)", words, lines, tt.wantWords, tt.wantLines)
			}
		})
	}
}
