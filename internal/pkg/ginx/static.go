package ginx

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

type noDirFS struct {
	fs http.FileSystem
}

type DirOptions struct {
	ShowList  bool
	SPA       bool
	IndexName string
	// [AgentDesk-BEGIN] 这些前缀下的路径「先尝试读文件，读不到就 404」，
	// 绝不回退到 index.html。
	//
	// 为什么需要它：SPA 兜底是为了让前端路由刷新可用，但静态资源目录
	// （/_next/）下的文件缺失时如果也回退成 index.html，浏览器会把 HTML
	// 当 JS/CSS 执行，报出与真实原因毫不相关的错误。将它们排除后，
	// 缺失资源会明确返回 404。
	//
	// 注意与 SPAOptions.NotFoundPrefixes 的区别：
	//   · NotFoundPrefixes：连文件都不尝试读，直接 404（用于 /api/）
	//   · FallbackExcludePrefixes：先读文件，读不到才 404（用于 /_next/）
	FallbackExcludePrefixes []string
	// [AgentDesk-END]
}

type SPAOptions struct {
	Root             string
	EmbeddedFS       fs.FS
	EmbeddedRoot     string
	DirOptions       DirOptions
	NotFoundPrefixes []string
	NotFoundHandler  gin.HandlerFunc
}

func StaticFiles(root string) http.FileSystem {
	return noDirFS{fs: http.Dir(root)}
}

func (n noDirFS) Open(name string) (http.File, error) {
	file, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, os.ErrNotExist
	}
	return file, nil
}

func HandleSPA(engine *gin.Engine, options SPAOptions) gin.HandlerFunc {
	handler := NewSPAHandler(options.Root, options.EmbeddedFS, options.EmbeddedRoot, options.DirOptions)
	engine.GET("/", handler)
	engine.HEAD("/", handler)
	engine.NoRoute(func(ctx *gin.Context) {
		for _, prefix := range options.NotFoundPrefixes {
			if strings.HasPrefix(ctx.Request.URL.Path, prefix) {
				if options.NotFoundHandler != nil {
					options.NotFoundHandler(ctx)
					return
				}
				ctx.AbortWithStatus(http.StatusNotFound)
				return
			}
		}
		handler(ctx)
	})
	return handler
}

func NewSPAHandler(root string, embeddedFS fs.FS, embeddedRoot string, options DirOptions) gin.HandlerFunc {
	if _, err := os.Stat(path.Join(root, options.IndexName)); err == nil {
		return DirHandler(http.Dir(root), options)
	}

	spaFS, err := fs.Sub(embeddedFS, embeddedRoot)
	if err != nil {
		slog.Error("failed to load embedded SPA files", slog.Any("err", err))
		return func(ctx *gin.Context) {
			ctx.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	return DirHandler(http.FS(spaFS), options)
}

func DirHandler(fileSystem http.FileSystem, options DirOptions) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		name := path.Clean(ctx.Request.URL.Path)
		if name == "." || name == "/" {
			name = "/" + options.IndexName
		}

		file, err := fileSystem.Open(name)
		if err == nil {
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				ctx.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if info.IsDir() {
				if options.IndexName != "" {
					indexName := path.Join(name, options.IndexName)
					indexFile, err := fileSystem.Open(indexName)
					if err == nil {
						defer indexFile.Close()
						indexInfo, statErr := indexFile.Stat()
						if statErr != nil {
							ctx.AbortWithStatus(http.StatusInternalServerError)
							return
						}
						if !indexInfo.IsDir() {
							serveFile(ctx, indexName, indexFile, indexInfo)
							return
						}
					}
					if err != nil && !os.IsNotExist(err) {
						ctx.AbortWithStatus(http.StatusInternalServerError)
						return
					}
				}
				if !options.ShowList {
					ctx.AbortWithStatus(http.StatusNotFound)
					return
				}
			} else {
				serveFile(ctx, name, file, info)
				return
			}
		} else if !os.IsNotExist(err) {
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		// [AgentDesk-BEGIN] Next.js 静态切片兼容与防御：
		// 1. Next.js 客户端软路由切片请求形如 /dashboard/register.txt，
		//    但在 static export (trailingSlash: true) 模式下，切片实际文件是 /dashboard/register/index.txt。
		//    若按原路径不存在，尝试探测对应目录下的 index.txt。
		if strings.HasSuffix(name, ".txt") {
			altIndexTxt := strings.TrimSuffix(name, ".txt") + "/index.txt"
			if altFile, altErr := fileSystem.Open(altIndexTxt); altErr == nil {
				defer altFile.Close()
				if altInfo, statErr := altFile.Stat(); statErr == nil && !altInfo.IsDir() {
					serveFile(ctx, altIndexTxt, altFile, altInfo)
					return
				}
			}
			// 2. 若仍找不到，.txt 切片绝不可回退到 index.html（否则客户端把 HTML 当 RSC flight 流解析直接抛错卡死，导致用户点击呈现死寂无反应）
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
		// [AgentDesk-END]

		if options.SPA && options.IndexName != "" {
			// [AgentDesk-BEGIN] 静态资源目录不回退 index.html
			for _, prefix := range options.FallbackExcludePrefixes {
				if prefix != "" && strings.HasPrefix(name, prefix) {
					ctx.AbortWithStatus(http.StatusNotFound)
					return
				}
			}
			// [AgentDesk-END]
			indexName := "/" + options.IndexName
			indexFile, err := fileSystem.Open(indexName)
			if err != nil {
				if os.IsNotExist(err) {
					ctx.AbortWithStatus(http.StatusNotFound)
					return
				}
				ctx.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			defer indexFile.Close()
			indexInfo, err := indexFile.Stat()
			if err != nil {
				ctx.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			serveFile(ctx, indexName, indexFile, indexInfo)
			return
		}

		ctx.AbortWithStatus(http.StatusNotFound)
	}
}

func serveFile(ctx *gin.Context, name string, file http.File, info os.FileInfo) {
	// [AgentDesk-BEGIN] 显式声明缓存策略
	if ct := cacheControlFor(name); ct != "" {
		ctx.Header("Cache-Control", ct)
	}
	// [AgentDesk-END]
	http.ServeContent(ctx.Writer, ctx.Request, name, info.ModTime(), file)
}

// [AgentDesk-BEGIN] 静态资源缓存策略
//
// 背景（实测踩坑）：原来不设置 Cache-Control，只由 ServeContent 提供 Last-Modified，
// 浏览器会对 HTML 做"启发式缓存"（按距上次修改的时间推算有效期）。后果是：
// 前端重新构建后，浏览器仍拿旧 HTML，而它引用的旧 chunk 已经不存在 ——
// 页面白屏或抛出与真实原因无关的 JS 异常，且普通刷新未必生效。
//
// 按 Next.js 惯例显式区分：
//   · /_next/static/*  文件名自带内容哈希，可长期强缓存（内容变了文件名就变）
//   · *.html           入口文件必须每次校验，否则用户会被钉在旧版本上
func cacheControlFor(name string) string {
	switch {
	case strings.HasPrefix(name, "/_next/static/"):
		return "public, max-age=31536000, immutable"
	case strings.HasSuffix(name, ".html"):
		return "no-cache, must-revalidate"
	default:
		return "public, max-age=3600"
	}
}

// [AgentDesk-END]
