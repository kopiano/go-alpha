package controller

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go-alpha/response"
)

const docDir = "/app/assets/docs"

type docFile struct {
	Tag     string `json:"tag"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Path    string `json:"path"`
	Time    string `json:"time"`
}

type tagGroup struct {
	Tag   string    `json:"tag"`
	Count int       `json:"count"`
	Items []docFile `json:"items"`
}

type DocController struct{}

func (d *DocController) Save(ctx *gin.Context) {
	var form struct {
		Tag     string `json:"tag" binding:"required"`
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，tag、title、content 为必填", ctx)
		return
	}

	tag := strings.TrimSpace(form.Tag)
	if tag == "" || strings.ContainsAny(tag, "/\\.") {
		response.Failed("tag 格式不正确", ctx)
		return
	}

	dir := filepath.Join(docDir, tag)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Doc.Save: MkdirAll failed", "error", err)
		response.Failed("创建目录失败", ctx)
		return
	}

	filename := strings.ReplaceAll(form.Title, " ", "_") + ".md"
	savePath := filepath.Join(dir, filename)

	if err := os.WriteFile(savePath, []byte(form.Content), 0644); err != nil {
		slog.Error("Doc.Save: WriteFile failed", "error", err)
		response.Failed("保存文件失败", ctx)
		return
	}

	response.Success("保存成功", gin.H{
		"path": filepath.Join("/docs", tag, filename),
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}, ctx)
}

func (d *DocController) List(ctx *gin.Context) {
	entries, err := os.ReadDir(docDir)
	if err != nil {
		if os.IsNotExist(err) {
			response.Success("ok", []tagGroup{}, ctx)
			return
		}
		slog.Error("Doc.List: ReadDir failed", "error", err)
		response.Failed("读取目录失败", ctx)
		return
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		groups []tagGroup
	)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tag := entry.Name()
		tagDir := filepath.Join(docDir, tag)

		wg.Add(1)
		go func(tag, tagDir string) {
			defer wg.Done()
			files, err := os.ReadDir(tagDir)
			if err != nil {
				return
			}
			var items []docFile
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
					continue
				}
				content, err := os.ReadFile(filepath.Join(tagDir, f.Name()))
				if err != nil {
					continue
				}
				info, err := f.Info()
				if err != nil {
					continue
				}
				title := strings.TrimSuffix(f.Name(), ".md")
				items = append(items, docFile{
					Tag:     tag,
					Title:   title,
					Content: string(content),
					Path:    filepath.Join("/docs", tag, f.Name()),
					Time:    info.ModTime().Format("2006-01-02 15:04:05"),
				})
			}
			if len(items) == 0 {
				return
			}
			mu.Lock()
			groups = append(groups, tagGroup{
				Tag:   tag,
				Count: len(items),
				Items: items,
			})
			mu.Unlock()
		}(tag, tagDir)
	}

	wg.Wait()

	if groups == nil {
		groups = []tagGroup{}
	}
	response.Success("ok", groups, ctx)
}
