// Package cardimage renders the welcome and marriage images, using the
// assets in runtime/assets (bg.png, heart.png, font.ttf).
package cardimage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

const (
	assetDir   = "runtime/assets/"
	avatarSize = 192
	fontSize   = 40
)

var (
	baseColor   = color.RGBA{54, 57, 63, 255}
	ringColor   = color.RGBA{255, 255, 255, 255}
	textBgColor = color.RGBA{0, 0, 0, 200}
	httpClient  = &http.Client{Timeout: 15 * time.Second}
)

// Welcome renders a welcome card with the member's avatar in the middle.
func Welcome(user *discordgo.User, guildName string) ([]byte, error) {
	bg, face, err := loadCommon()
	if err != nil {
		return nil, err
	}
	avatar, err := fetchAvatar(user)
	if err != nil {
		return nil, err
	}

	canvas := newCanvas(bg)
	b := canvas.Bounds()
	pasteAvatar(canvas, avatar, image.Pt((b.Dx()-avatarSize)/2, (b.Dy()-avatarSize)/2))

	drawTextBox(canvas, face, "Welcome to "+guildName+"!", false)
	drawTextBox(canvas, face, "@"+user.Username, true)
	return encode(canvas)
}

// Marriage renders a "marriage certificate" with both avatars and a heart.
func Marriage(first, second *discordgo.User) ([]byte, error) {
	bg, face, err := loadCommon()
	if err != nil {
		return nil, err
	}
	heart, err := loadImage(assetDir + "heart.png")
	if err != nil {
		return nil, err
	}
	firstAvatar, err := fetchAvatar(first)
	if err != nil {
		return nil, err
	}
	secondAvatar, err := fetchAvatar(second)
	if err != nil {
		return nil, err
	}

	canvas := newCanvas(bg)
	b := canvas.Bounds()
	y := (b.Dy() - avatarSize) / 2
	pasteAvatar(canvas, firstAvatar, image.Pt((b.Dx()-avatarSize)/5, y))
	pasteAvatar(canvas, secondAvatar, image.Pt((b.Dx()-avatarSize)/5*4, y))

	hb := heart.Bounds()
	heartAt := image.Pt((b.Dx()-hb.Dx())/2, (b.Dy()-hb.Dy())/2)
	draw.Draw(canvas, hb.Sub(hb.Min).Add(heartAt), heart, hb.Min, draw.Over)

	drawTextBox(canvas, face, "@"+first.Username+" x @"+second.Username, true)
	return encode(canvas)
}

func loadCommon() (image.Image, font.Face, error) {
	bg, err := loadImage(assetDir + "bg.png")
	if err != nil {
		return nil, nil, err
	}
	fontData, err := os.ReadFile(assetDir + "font.ttf")
	if err != nil {
		return nil, nil, err
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return nil, nil, err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: fontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, nil, err
	}
	return bg, face, nil
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func fetchAvatar(user *discordgo.User) (image.Image, error) {
	url := discordgo.EndpointDefaultUserAvatar(user.DefaultAvatarIndex())
	if user.Avatar != "" {
		// Always request a static PNG, even for animated avatars
		url = discordgo.EndpointUserAvatar(user.ID, user.Avatar) + "?size=256"
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar download returned %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func newCanvas(bg image.Image) *image.RGBA {
	b := bg.Bounds()
	canvas := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{baseColor}, image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), bg, b.Min, draw.Over)
	return canvas
}

// pasteAvatar draws a circular avatar with a white ring at the given top-left point.
func pasteAvatar(canvas *image.RGBA, avatar image.Image, at image.Point) {
	scaled := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), avatar, avatar.Bounds(), draw.Src, nil)

	center := image.Pt(at.X+avatarSize/2, at.Y+avatarSize/2)
	r := avatarSize / 2
	ring := &circle{center: center, r: r + 2}
	draw.DrawMask(canvas, ring.Bounds(), &image.Uniform{ringColor}, image.Point{}, ring, ring.Bounds().Min, draw.Over)
	fill := &circle{center: center, r: r}
	draw.DrawMask(canvas, fill.Bounds(), &image.Uniform{baseColor}, image.Point{}, fill, fill.Bounds().Min, draw.Over)

	mask := &circle{center: image.Pt(r, r), r: r}
	draw.DrawMask(canvas, image.Rect(at.X, at.Y, at.X+avatarSize, at.Y+avatarSize), scaled, image.Point{}, mask, image.Point{}, draw.Over)
}

// drawTextBox draws centered white text on a translucent black box,
// at the top (bottom == false) or bottom of the canvas.
func drawTextBox(canvas *image.RGBA, face font.Face, text string, bottom bool) {
	b := canvas.Bounds()
	metrics := face.Metrics()
	textW := font.MeasureString(face, text).Ceil()
	textH := (metrics.Ascent + metrics.Descent).Ceil()

	x := (b.Dx() - textW) / 2
	y := 10
	if bottom {
		y = b.Dy() - 10 - textH
	}
	box := image.Rect(x, y, x+textW, y+textH)
	draw.Draw(canvas, box, &image.Uniform{textBgColor}, image.Point{}, draw.Over)

	d := &font.Drawer{
		Dst:  canvas,
		Src:  image.White,
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y) + metrics.Ascent},
	}
	d.DrawString(text)
}

func encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// circle is an anti-aliased circular alpha mask.
type circle struct {
	center image.Point
	r      int
}

func (c *circle) ColorModel() color.Model { return color.AlphaModel }

func (c *circle) Bounds() image.Rectangle {
	return image.Rect(c.center.X-c.r, c.center.Y-c.r, c.center.X+c.r, c.center.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
	dx := float64(x-c.center.X) + 0.5
	dy := float64(y-c.center.Y) + 0.5
	dist := dx*dx + dy*dy
	rr := float64(c.r)
	switch {
	case dist <= (rr-1)*(rr-1):
		return color.Alpha{255}
	case dist >= rr*rr:
		return color.Alpha{0}
	default:
		// Linear falloff over the last pixel for smooth edges
		frac := (rr*rr - dist) / (rr*rr - (rr-1)*(rr-1))
		return color.Alpha{uint8(frac * 255)}
	}
}
