package fun

import (
	"math/rand"
	"regexp"

	"github.com/bwmarrin/discordgo"
)

var kaomojis = []string{
	"(ᵘʷᵘ)",
	"(ᵘﻌᵘ)",
	"(◡ ω ◡)",
	"(◡ ꒳ ◡)",
	"(◡ w ◡)",
	"(◡ ሠ ◡)",
	"(˘ω˘)",
	"(⑅˘꒳˘)",
	"(˘ᵕ˘)",
	"(˘ሠ˘)",
	"(˘³˘)",
	"(˘ε˘)",
	"(´˘`)",
	"(´꒳`)",
	"(˘ ˘ ˘)⭜",
	"( ᴜ ω ᴜ )",
	"( ´ω` )۶",
	"(„ᵕᴗᵕ„)",
	"(*ฅ́˘ฅ̀*)",
	"(ㅅꈍ ˘ ꈍ)",
	"(⑅˘꒳˘)",
	"( ｡ᵘ ᵕ ᵘ ｡)",
	"( ᵘ ꒳ ᵘ ✼)",
	"( ˘ᴗ˘ )",
	"(˯ ᵘ ꒳ ᵘ ˯)",
	"(ᵘᆸᵘ)⭜",
	"(。U ω U。)",
	"(。U⁄ ⁄ω⁄ ⁄ U。)",
	"(U ᵕ U❁)",
	"(U ﹏ U)",
	"(⁄˘⁄ ⁄ ω⁄ ⁄ ˘⁄)♡",
	"( ͡U ω ͡U )",
	"( ͡o ᵕ ͡o )",
	"( ͡o ꒳ ͡o )",
	"(❀˘꒳˘)♡(˘꒳˘❀)",
	"( ˊ.ᴗˋ )",
}

// replacements = {
//             r"(?:r|l)": "w",
//             r"(?:R|L)/g": "W",
//             r"n([aeiou])": "ny",
//             r"N([aeiou])": "Ny",
//             r"N([AEIOU])": "Ny",
//             r"ove": "uv",
//             r"o": "owo",
//             r"O": "OwO",
//             r"u": "uwu",
//             r"U": "UwU",
//         }

var owoReplacements = []struct {
	old *regexp.Regexp
	new string
}{
	{regexp.MustCompile(`(?:r|l)`), "w"},
	{regexp.MustCompile(`(?:R|L)`), "W"},
	{regexp.MustCompile(`n([aeiou])`), "ny"},
	{regexp.MustCompile(`N([aeiou])`), "Ny"},
	{regexp.MustCompile(`N([AEIOU])`), "Ny"},
	{regexp.MustCompile(`ove`), "uv"},
	{regexp.MustCompile(`o`), "owo"},
	{regexp.MustCompile(`O`), "OwO"},
	{regexp.MustCompile(`u`), "uwu"},
	{regexp.MustCompile(`U`), "UwU"},
}

func Owoify(input string) string {
	output := input
	for _, replacement := range owoReplacements {
		output = replacement.old.ReplaceAllString(output, replacement.new)
	}
	return output + " " + kaomojis[rand.Intn(len(kaomojis))]
}

func OwoCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	if len(args) == 0 {
		ctx.ChannelMessageSend(msg.ChannelID, "Please provide a message to owoify!")
		return
	}
	input := ""
	for _, arg := range args {
		input += arg + " "
	}
	owoified := Owoify(input)
	ctx.ChannelMessageSendReply(msg.ChannelID, owoified, msg.Reference())
}
