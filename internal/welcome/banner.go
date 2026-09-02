package welcome

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
)

func GenerateBanner(username, avatarURL, serverName string, memberCount int) ([]byte, error) {
	const W, H = 1000, 400
	dc := gg.NewContext(W, H)

	// Background gradient like MEE6/Carl: #0055ff -> #00d4ff
	for y := 0; y < H; y++ {
		t := float64(y) / float64(H)
		r := uint8(0x00 + t*0x00)
		g := uint8(0x55 + t*(0xd4-0x55))
		b := uint8(0xff + t*(0xff-0xff))
		dc.SetRGB(float64(r)/255, float64(g)/255, float64(b)/255)
		dc.DrawLine(0, float64(y), float64(W), float64(y))
		dc.Stroke()
	}

	// Card overlay
	dc.SetRGBA(1, 1, 1, 0.15)
	dc.DrawRoundedRectangle(30, 30, W-60, H-60, 20)
	dc.Fill()

	// Avatar circle 160x160 at 100,120
	avatarImg := fetchAvatar(avatarURL, 160)
	if avatarImg != nil {
		// Clip circle
		dc.DrawCircle(180, 200, 80)
		dc.Clip()
		dc.DrawImageAnchored(avatarImg, 180, 200, 0.5, 0.5)
		dc.ResetClip()
		// Border
		dc.SetRGBA(1, 1, 1, 0.9)
		dc.SetLineWidth(4)
		dc.DrawCircle(180, 200, 80)
		dc.Stroke()
	} else {
		dc.SetRGB(1, 1, 1)
		dc.DrawCircle(180, 200, 80)
		dc.Fill()
		dc.SetRGB(0, 0, 0)
		dc.DrawStringAnchored(username[:1], 180, 200, 0.5, 0.35)
	}

	// Text: Welcome @username
	dc.SetRGB(1, 1, 1)
	if err := dc.LoadFontFace("C:\\Windows\\Fonts\\segoeuib.ttf", 42); err != nil {
		dc.LoadFontFace("C:\\Windows\\Fonts\\arialbd.ttf", 42)
	}
	dc.DrawStringAnchored("Welcome", 320, 140, 0, 0.5)
	if err := dc.LoadFontFace("C:\\Windows\\Fonts\\segoeui.ttf", 28); err != nil {
		dc.LoadFontFace("C:\\Windows\\Fonts\\arial.ttf", 28)
	}
	// Username
	dc.SetRGBA(1, 1, 1, 0.95)
	// Truncate long names
	if len(username) > 20 {
		username = username[:20] + "…"
	}
	dc.DrawStringAnchored(username, 320, 190, 0, 0.5)
	// Server + member count
	dc.SetRGBA(1, 1, 1, 0.75)
	if err := dc.LoadFontFace("C:\\Windows\\Fonts\\segoeui.ttf", 18); err != nil {
		dc.LoadFontFace("C:\\Windows\\Fonts\\arial.ttf", 18)
	}
	dc.DrawStringAnchored(serverName+" • Member #"+itoa(memberCount), 320, 230, 0, 0.5)
	dc.DrawStringAnchored("You are the "+ordinal(memberCount)+" member!", 320, 260, 0, 0.5)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fetchAvatar(urlStr string, size int) image.Image {
	if urlStr == "" {
		return nil
	}
	// Discord avatar URL may be webp, try as is
	if !strings.HasPrefix(urlStr, "http") {
		return nil
	}
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	img, err := imaging.Decode(resp.Body)
	if err != nil {
		// try png/jpeg fallback
		img, _, err = image.Decode(resp.Body)
		if err != nil {
			return nil
		}
	}
	img = imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
	// Make circle via gg clip later, just return square
	_ = png.Encode
	return img
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 != 11 && n%100 != 12 && n%100 != 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return itoa(n) + suffix
}
