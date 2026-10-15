package api

import (
	"bytes"
	"dwc-server-advanced/common"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestProcessAvatarCropsAndScales(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 800, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 800; x++ {
			source.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, source, nil); err != nil {
		t.Fatal(err)
	}

	out, err := processAvatar(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal("output is not PNG:", err)
	}
	if b := decoded.Bounds(); b.Dx() != profileAvatarSize || b.Dy() != profileAvatarSize {
		t.Fatalf("got %dx%d, want %dx%d", b.Dx(), b.Dy(), profileAvatarSize, profileAvatarSize)
	}
	r, _, _, _ := decoded.At(128, 128).RGBA()
	if r>>8 < 180 {
		t.Fatalf("colour lost while scaling: r=%d", r>>8)
	}
}

func TestProcessAvatarKeepsSmallImages(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 64, 90)))
	out, err := processAvatar(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := png.Decode(bytes.NewReader(out))
	if decoded.Bounds().Dx() != 64 || decoded.Bounds().Dy() != 64 {
		t.Fatalf("got %v", decoded.Bounds())
	}
}

func TestProcessAvatarRejectsOther(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), []byte("GIF89a")} {
		if _, err := processAvatar(data); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
}

func TestCleanProfileBio(t *testing.T) {
	got, err := cleanProfileBio("  hi\r\nthere\x00\n\n\n\nbye  ")
	if err != nil || got != "hi\nthere\n\nbye" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := cleanProfileBio(strings.Repeat("é", profileBioMaxLength+1)); err == nil {
		t.Fatal("accepted a bio that is too long")
	}
	if _, err := cleanProfileBio(strings.Repeat("é", profileBioMaxLength)); err != nil {
		t.Fatal(err)
	}
}

func TestProfilesToggleIsLive(t *testing.T) {
	before := common.DefaultConfig()
	after := before
	after.Profiles.Enabled = false
	if restartRequired(before, after) {
		t.Fatal("turning profiles off should not need a restart")
	}
}
