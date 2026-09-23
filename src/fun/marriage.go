package fun

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/cardimage"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/database"
)

const (
	proposalTimeout = 60 * time.Second
	divorceTimeout  = 30 * time.Second
	ringThumbnail   = "https://em-content.zobj.net/thumbs/120/twitter/348/ring_1f48d.png"
)

type marriage struct {
	first, second string
	start         time.Time
}

func ShipCommand(c *command.Ctx) {
	first, second := c.User("first"), c.User("second")
	c.Reply(fmt.Sprintf("Oh look %s ships %s and %s together \nAww... 🛳️",
		c.Author.Mention(), first.Mention(), second.Mention()))
}

func MarryCommand(c *command.Ctx) {
	target := c.User("user")
	switch {
	case target.ID == c.Author.ID:
		c.ReplyPrivate("What are you trying to do...?")
		return
	case target.Bot:
		c.ReplyPrivate("I'm flattered, but bots can't get married.")
		return
	}

	if m, err := findMarriage(c.GuildID, c.Author.ID); err != nil {
		c.ReplyPrivate("Something went wrong checking the marriage records :(")
		return
	} else if m != nil {
		c.ReplyPrivate(fmt.Sprintf("You already have your soul attached to %s. What are you doing?", partnerMention(m, c.Author.ID)))
		return
	}
	if m, err := findMarriage(c.GuildID, target.ID); err != nil {
		c.ReplyPrivate("Something went wrong checking the marriage records :(")
		return
	} else if m != nil {
		c.ReplyPrivate(fmt.Sprintf("Sorry, but their soul has already been attached to %s. I am magic, but I can't do anything for you", partnerMention(m, target.ID)))
		return
	}

	proposal := &discordgo.MessageSend{
		Content: fmt.Sprintf("%s, you have %d seconds to answer!", target.Mention(), int(proposalTimeout.Seconds())),
		Embeds: []*discordgo.MessageEmbed{{
			Title:     "Marriage Proposal!",
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: ringThumbnail},
			Fields: []*discordgo.MessageEmbedField{
				command.Field("Member", "```"+c.Author.Username+"```", false),
				command.Field("Has proposed", "```"+target.Username+"```", false),
				command.Field("to a relationship!", "Will "+target.Username+" accept?", false),
			},
		}},
	}
	answer, ok := c.Ask(proposal, target.ID, []command.Button{
		{Label: "Yes!", Style: discordgo.SuccessButton, Value: "yes"},
		{Label: "No", Style: discordgo.SecondaryButton, Value: "no"},
	}, proposalTimeout)
	if !ok {
		c.Reply("Oh no! They didn't seem to care :(")
		return
	}
	if answer != "yes" {
		c.Reply("Ouch. They said no... 💔")
		return
	}

	// Check again, in case either side got married while the proposal was open
	for _, id := range []string{c.Author.ID, target.ID} {
		if m, _ := findMarriage(c.GuildID, id); m != nil {
			c.Reply("Looks like someone got married in the meantime... 🤔")
			return
		}
	}

	_, err := database.Get().Exec(`INSERT INTO Marriage (GuildID, FirstSide, SecondSide, StartDate) VALUES (?, ?, ?, ?)`,
		c.GuildID, c.Author.ID, target.ID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		log.Default().Println("Error saving marriage: " + err.Error())
		c.Reply("Something went wrong writing the marriage records :(")
		return
	}

	sendWithCertificate(c, c.Author, target, fmt.Sprintf(
		"Yay! Congratulations, %s ❤️ %s. We wish they have a sweet time together! 💍",
		c.Author.Mention(), target.Mention()))
}

func DivorceCommand(c *command.Ctx) {
	m, err := findMarriage(c.GuildID, c.Author.ID)
	if err != nil {
		c.ReplyPrivate("Something went wrong checking the marriage records :(")
		return
	}
	if m == nil {
		c.ReplyPrivate("You are not in a marriage with anyone yet...")
		return
	}

	prompt := &discordgo.MessageSend{
		Content:         fmt.Sprintf("You are in a relationship. You know, the one between <@%s> and <@%s>...\nAre you really sure about this?", m.first, m.second),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
	answer, ok := c.Ask(prompt, c.Author.ID, []command.Button{
		{Label: "DO IT", Style: discordgo.DangerButton, Value: "yes"},
		{Label: "Never mind", Style: discordgo.SecondaryButton, Value: "no"},
	}, divorceTimeout)
	if !ok {
		c.Reply("You did not reply to the request")
		return
	}
	if answer != "yes" {
		c.Reply("Phew. Crisis averted ❤️")
		return
	}

	_, err = database.Get().Exec(`DELETE FROM Marriage WHERE GuildID = ? AND (FirstSide = ? OR SecondSide = ?)`,
		c.GuildID, c.Author.ID, c.Author.ID)
	if err != nil {
		log.Default().Println("Error deleting marriage: " + err.Error())
		c.Reply("Something went wrong writing the marriage records :(")
		return
	}
	c.Reply("Alright, I deleted your marriage records... 💔")
}

func MarriageCertCommand(c *command.Ctx) {
	c.Defer()
	subject := c.Author
	if u := c.User("user"); u != nil {
		subject = u
	}

	m, err := findMarriage(c.GuildID, subject.ID)
	if err != nil {
		c.Reply("Something went wrong checking the marriage records :(")
		return
	}
	if m == nil {
		if subject.ID == c.Author.ID {
			c.Reply("You are not in a marriage with anyone yet...")
		} else {
			c.Reply("The person in question is not in a marriage with anyone yet. Go get 'em!")
		}
		return
	}

	first, err1 := c.Session.User(m.first)
	second, err2 := c.Session.User(m.second)
	if err1 != nil || err2 != nil {
		c.Reply("I couldn't find one of the lovebirds :(")
		return
	}
	sendWithCertificate(c, first, second, fmt.Sprintf("Relationship between %s and %s, which is %s long!",
		first.Mention(), second.Mention(), humanDuration(time.Since(m.start))))
}

// sendWithCertificate sends a message with the marriage image, or just the text
// if the image can't be generated (e.g. missing assets).
func sendWithCertificate(c *command.Ctx, first, second *discordgo.User, content string) {
	ms := &discordgo.MessageSend{
		Content:         content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
	img, err := cardimage.Marriage(first, second)
	if err != nil {
		log.Default().Println("Error generating marriage image: " + err.Error())
	} else {
		ms.Files = []*discordgo.File{{Name: "marriage.png", ContentType: "image/png", Reader: bytes.NewReader(img)}}
	}
	c.Send(ms)
}

func findMarriage(guildID, userID string) (*marriage, error) {
	var m marriage
	var start string
	err := database.Get().QueryRow(`SELECT FirstSide, SecondSide, StartDate FROM Marriage
		WHERE GuildID = ? AND (FirstSide = ? OR SecondSide = ?)`, guildID, userID, userID).
		Scan(&m.first, &m.second, &start)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		log.Default().Println("Error querying marriages: " + err.Error())
		return nil, err
	}
	m.start, _ = time.Parse(time.RFC3339, start)
	return &m, nil
}

func partnerMention(m *marriage, userID string) string {
	if m.first == userID {
		return "<@" + m.second + ">"
	}
	return "<@" + m.first + ">"
}

func humanDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, plural(days, "day"))
	}
	if hours > 0 {
		parts = append(parts, plural(hours, "hour"))
	}
	if days == 0 && (minutes > 0 || hours == 0) {
		parts = append(parts, plural(minutes, "minute"))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
