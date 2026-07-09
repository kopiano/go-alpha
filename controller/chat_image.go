package controller

import (
	"bytes"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"image"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

const defaultChatImageMaxWidth = 1600
const defaultChatImageQuality = 82

func chatImageDir() string {
	if v := strings.TrimSpace(os.Getenv("CHAT_IMAGE_DIR")); v != "" {
		return v
	}
	return filepath.Join(".", "assets", "image")
}

func ChatImageDir() string {
	return chatImageDir()
}

func chatImageMaxWidth() int {
	if v := strings.TrimSpace(os.Getenv("CHAT_IMAGE_MAX_WIDTH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultChatImageMaxWidth
}

func chatImageQuality() float32 {
	if v := strings.TrimSpace(os.Getenv("CHAT_IMAGE_QUALITY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			return float32(n)
		}
	}
	return defaultChatImageQuality
}

func chatImageFileName(username string, ts int64) string {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		username = "user"
	}
	var b strings.Builder
	b.Grow(len(username) + len("image-.webp"))
	b.WriteString("image-")
	lastDash := false
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "image" || name == "" {
		return "image-user-" + strconv.FormatInt(ts, 10) + ".webp"
	}
	return name + "-" + strconv.FormatInt(ts, 10) + ".webp"
}

func saveChatImageAsWebp(savePath string, srcBytes []byte) error {
	img, err := decodeChatImage(srcBytes)
	if err != nil {
		return err
	}
	targetWidth := chatImageMaxWidth()
	if bounds := img.Bounds(); bounds.Dx() > targetWidth {
		img = imaging.Resize(img, targetWidth, 0, imaging.Lanczos)
	}
	dst, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer dst.Close()
	return webp.Encode(dst, img, &webp.Options{Lossless: false, Quality: chatImageQuality()})
}

func decodeChatImage(srcBytes []byte) (image.Image, error) {
	if img, err := webp.Decode(bytes.NewReader(srcBytes)); err == nil {
		return img, nil
	}
	img, _, err := image.Decode(bytes.NewReader(srcBytes))
	return img, err
}

func validateChatImageSize(file *multipart.FileHeader) bool {
	return file != nil && file.Size > 0 && file.Size <= 20*1024*1024
}

func init() {
	if err := os.MkdirAll(chatImageDir(), 0o755); err != nil {
		slog.Warn("failed to create chat image dir", "error", err)
	}
}
