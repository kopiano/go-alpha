package controller

import (
	"encoding/json"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func HotSearch(c *gin.Context) {
	cmd := exec.Command("python3", "cmd/weibo.py", "--json")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hot search"})
		return
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	c.JSON(http.StatusOK, result)
}
