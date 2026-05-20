package visualize

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/pkg/browser"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

const svgPlaceholder = "{{SVG_CONTENT}}"

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>GitLab CI Pipeline Visualization</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            overflow: hidden;
            background: #f5f5f5;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            width: 100vw;
            height: 100vh;
        }
        #controls {
            position: fixed;
            top: 12px;
            right: 12px;
            z-index: 10;
            display: flex;
            gap: 6px;
        }
        #controls button {
            width: 36px;
            height: 36px;
            border: 1px solid #ddd;
            border-radius: 6px;
            background: #fff;
            font-size: 18px;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        #controls button:hover { background: #f0f0f0; }
        #viewport {
            width: 100%;
            height: 100%;
            cursor: grab;
        }
        #viewport.grabbing { cursor: grabbing; }
        #canvas {
            transform-origin: 0 0;
        }
        #canvas svg {
            display: block;
        }
    </style>
</head>
<body>
    <div id="controls">
        <button onclick="zoomIn()" title="Zoom in">+</button>
        <button onclick="zoomOut()" title="Zoom out">&minus;</button>
        <button onclick="resetView()" title="Fit to screen">&#8859;</button>
    </div>
    <div id="viewport">
        <div id="canvas">` + svgPlaceholder + `</div>
    </div>
    <script>
        let scale = 1, panX = 0, panY = 0;
        let dragging = false, startX, startY;
        const viewport = document.getElementById('viewport');
        const canvas = document.getElementById('canvas');
        const svg = canvas.querySelector('svg');

        function applyTransform() {
            canvas.style.transform = 'translate(' + panX + 'px,' + panY + 'px) scale(' + scale + ')';
        }

        function fitToScreen() {
            if (!svg) return;
            const vw = viewport.clientWidth;
            const vh = viewport.clientHeight;
            const sw = svg.getBoundingClientRect().width / scale;
            const sh = svg.getBoundingClientRect().height / scale;
            scale = Math.min(vw / sw, vh / sh, 1) * 0.9;
            panX = (vw - sw * scale) / 2;
            panY = (vh - sh * scale) / 2;
            applyTransform();
        }

        function zoomIn() { zoomAt(1.25, viewport.clientWidth / 2, viewport.clientHeight / 2); }
        function zoomOut() { zoomAt(0.8, viewport.clientWidth / 2, viewport.clientHeight / 2); }
        function resetView() { fitToScreen(); }

        function zoomAt(factor, cx, cy) {
            const newScale = Math.min(Math.max(scale * factor, 0.1), 10);
            panX = cx - (cx - panX) * (newScale / scale);
            panY = cy - (cy - panY) * (newScale / scale);
            scale = newScale;
            applyTransform();
        }

        viewport.addEventListener('wheel', function(e) {
            e.preventDefault();
            const factor = e.deltaY < 0 ? 1.1 : 0.9;
            zoomAt(factor, e.clientX, e.clientY);
        }, { passive: false });

        viewport.addEventListener('mousedown', function(e) {
            dragging = true;
            startX = e.clientX - panX;
            startY = e.clientY - panY;
            viewport.classList.add('grabbing');
        });

        window.addEventListener('mousemove', function(e) {
            if (!dragging) return;
            panX = e.clientX - startX;
            panY = e.clientY - startY;
            applyTransform();
        });

        window.addEventListener('mouseup', function() {
            dragging = false;
            viewport.classList.remove('grabbing');
        });

        fitToScreen();
    </script>
</body>
</html>`

type visualizeServer struct {
	io         *iostreams.IOStreams
	svgData    []byte
	listenAddr string
	pageData   []byte
}

func (s *visualizeServer) Run(ctx context.Context) error {
	// Pre-render the full HTML page to avoid format string issues with SVG content.
	s.pageData = []byte(strings.Replace(htmlTemplate, svgPlaceholder, string(s.svgData), 1))

	l, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	url := fmt.Sprintf("http://%s", l.Addr())
	fmt.Fprintf(s.io.StdErr, "Visualization server listening on %s\n", url)
	fmt.Fprintln(s.io.StdErr, "Press Ctrl+C to stop the server.")

	if err := browser.OpenURL(url); err != nil {
		s.io.LogError("Failed to open browser:", err)
		fmt.Fprintf(s.io.StdErr, "Open %s manually in your browser.\n", url)
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(s.handle),
	}

	go func() {
		<-ctx.Done()
		srv.Close() //nolint:errcheck
	}()

	err = srv.Serve(l)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *visualizeServer) handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(s.pageData) //nolint:errcheck
}
