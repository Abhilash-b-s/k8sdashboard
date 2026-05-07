package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
)

// StreamLogs streams pod logs using Server-Sent Events
func StreamLogs(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	podName := c.Param("pod")
	container := c.Query("container")

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// Create log stream request
	tailLines := int64(100)
	podLogOpts := &corev1.PodLogOptions{
		Follow:    true,
		TailLines: &tailLines,
	}
	if container != "" {
		podLogOpts.Container = container
	}

	req := k8s.Clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		c.SSEvent("error", fmt.Sprintf("Failed to get logs: %v", err))
		return
	}
	defer stream.Close()

	// Create context for cancellation
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Stream logs using goroutine
	go func() {
		<-ctx.Done()
		stream.Close()
	}()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return
			}
			c.SSEvent("log", line)
			c.Writer.Flush()
		}
	}
}
