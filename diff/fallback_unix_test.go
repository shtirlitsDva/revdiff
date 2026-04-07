//go:build !windows

package diff

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unix-only because syscall.Mkfifo and reliable os.Symlink semantics
// require POSIX. Windows symlinks need elevation/Developer Mode and
// have no FIFO equivalent. The directory case is covered cross-platform
// by TestReadFileAsContext_NonRegularFile_Directory in fallback_test.go.

func TestReadFileAsContext_FifoAndSymlink(t *testing.T) {
	dir := t.TempDir()

	t.Run("fifo returns placeholder", func(t *testing.T) {
		fifoPath := filepath.Join(dir, "test.fifo")
		require.NoError(t, syscall.Mkfifo(fifoPath, 0o600))

		lines, err := readFileAsContext(fifoPath)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		assert.Equal(t, "(not a regular file)", lines[0].Content)
		assert.Equal(t, ChangeContext, lines[0].ChangeType)
	})

	t.Run("symlink to fifo returns placeholder", func(t *testing.T) {
		fifoPath := filepath.Join(dir, "target.fifo")
		require.NoError(t, syscall.Mkfifo(fifoPath, 0o600))
		linkPath := filepath.Join(dir, "link-to-fifo")
		require.NoError(t, os.Symlink(fifoPath, linkPath))

		lines, err := readFileAsContext(linkPath)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		assert.Equal(t, "(not a regular file)", lines[0].Content)
	})

	t.Run("broken symlink returns placeholder", func(t *testing.T) {
		linkPath := filepath.Join(dir, "broken-link")
		require.NoError(t, os.Symlink("/nonexistent/target", linkPath))

		lines, err := readFileAsContext(linkPath)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		assert.Equal(t, "(broken symlink)", lines[0].Content)
		assert.Equal(t, ChangeContext, lines[0].ChangeType)
		assert.Equal(t, 1, lines[0].OldNum)
		assert.Equal(t, 1, lines[0].NewNum)
	})

	t.Run("symlink to regular file reads normally", func(t *testing.T) {
		realFile := filepath.Join(dir, "real.txt")
		require.NoError(t, os.WriteFile(realFile, []byte("hello\n"), 0o600))
		linkPath := filepath.Join(dir, "link-to-real")
		require.NoError(t, os.Symlink(realFile, linkPath))

		lines, err := readFileAsContext(linkPath)
		require.NoError(t, err)
		require.Len(t, lines, 1)
		assert.Equal(t, "hello", lines[0].Content)
	})
}
