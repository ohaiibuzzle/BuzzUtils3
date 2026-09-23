package command

import (
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

func TestNextToken(t *testing.T) {
	tok, rest := nextToken("  sauceplz   hu tao  + ganyu ")
	if tok != "sauceplz" || rest != "hu tao  + ganyu " {
		t.Errorf("got %q, %q", tok, rest)
	}
	if tok, rest := nextToken("ping"); tok != "ping" || rest != "" {
		t.Errorf("got %q, %q", tok, rest)
	}
}

func TestSnowflake(t *testing.T) {
	for in, want := range map[string]snowflake.ID{
		"<@123456789012345678>":  123456789012345678,
		"<@!123456789012345678>": 123456789012345678,
		"<@&123456789012345678>": 123456789012345678,
		"<#123456789012345678>":  123456789012345678,
		"123456789012345678":     123456789012345678,
		"hello":                  0,
		"<@12>":                  0,
	} {
		if got := parseSnowflake(in); got != want {
			t.Errorf("parseSnowflake(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParsePrefixArgs(t *testing.T) {
	options := []Option{
		{Type: discord.ApplicationCommandOptionTypeInt, Name: "amount", Required: true},
		{Type: discord.ApplicationCommandOptionTypeString, Name: "text"},
	}

	c := &Ctx{args: map[string]any{}}
	if err := parsePrefixArgs(c, options, "5 hello there  world"); err != nil {
		t.Fatal(err)
	}
	if n, ok := c.Int("amount"); !ok || n != 5 {
		t.Errorf("amount = %v, %v", n, ok)
	}
	if c.String("text") != "hello there  world" {
		t.Errorf("text = %q, the last string option should take the rest of the message", c.String("text"))
	}

	c = &Ctx{args: map[string]any{}}
	if err := parsePrefixArgs(c, options, ""); err == nil {
		t.Error("expected an error for a missing required argument")
	}

	c = &Ctx{args: map[string]any{}}
	if err := parsePrefixArgs(c, options, "five"); err == nil {
		t.Error("expected an error for a non-numeric integer")
	}
	if _, ok := c.Int("amount"); ok {
		t.Error("invalid arguments should not be stored")
	}
}

func TestField(t *testing.T) {
	f := Field("", "", false)
	if f.Name == "" || f.Value != "N/A" {
		t.Errorf("empty fields should be filled in, got %+v", f)
	}
	long := make([]rune, 2000)
	for i := range long {
		long[i] = 'a'
	}
	if l := len([]rune(Field("x", string(long), false).Value)); l > 1024 {
		t.Errorf("field value should be truncated to 1024, got %d", l)
	}
}
