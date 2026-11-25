package component

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func assetLockHelperCommand(t *testing.T, root, mode string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestAssetLockProcessHelper$")
	cmd.Env = append(os.Environ(), "HB_ASSET_LOCK_HELPER="+mode, "HB_ASSET_LOCK_ROOT="+root,
		"XDG_CACHE_HOME="+t.TempDir(), "TMPDIR="+t.TempDir(), "TMP="+t.TempDir(), "TEMP="+t.TempDir())
	return cmd
}

func assetLockTryProcess(t *testing.T, root, mode, want string) {
	t.Helper()
	output, err := assetLockHelperCommand(t, root, mode).CombinedOutput()
	if err != nil || !strings.Contains(string(output), want+"\n") {
		t.Fatalf("lock subprocess %s: %v, output %s, want %s", mode, err, output, want)
	}
}

func assetLockHoldProcess(t *testing.T, root, mode string) (*exec.Cmd, io.WriteCloser) {
	t.Helper()
	cmd := assetLockHelperCommand(t, root, mode)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	ready := make(chan string, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			line += err.Error()
		}
		ready <- line
	}()
	select {
	case line := <-ready:
		if line != "READY\n" {
			t.Fatalf("lock child did not acquire its lock: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("lock child did not report acquisition")
	}
	return cmd, stdin
}

func TestAssetSessionIndependentOfCacheEnvironment(t *testing.T) {
	root := t.TempDir()
	unlock, err := LockAssetSession(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	assetLockTryProcess(t, root, "try-session", "BUSY")
	assetLockTryProcess(t, t.TempDir(), "try-session", "ACQUIRED")
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err == nil {
		assetLockTryProcess(t, alias, "try-session", "BUSY")
	} else {
		t.Logf("symlink alias case unavailable: %v", err)
	}
}

func TestAssetSessionReleasedOnProcessExit(t *testing.T) {
	root := t.TempDir()
	cmd, _ := assetLockHoldProcess(t, root, "hold-session")
	if release, err := LockAssetSession(root, "test"); err == nil {
		release()
		t.Fatal("parent acquired child's active session")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	unlock, err := LockAssetSession(root, "test")
	if err != nil {
		t.Fatalf("OS did not release terminated process session: %v", err)
	}
	unlock()
}

func TestAssetTreeSharedReadersExcludeWriterAcrossProcesses(t *testing.T) {
	root := t.TempDir()
	cmd, stdin := assetLockHoldProcess(t, root, "hold-reader")
	unlock, err := LockAssetTree(context.Background(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	assetLockTryProcess(t, root, "try-reader", "ACQUIRED")
	assetLockTryProcess(t, root, "try-writer", "BUSY")
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if release, err := LockAssetTree(ctx, root, false); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("waiting writer did not respect cancellation: %v", err)
	}
	unlock()
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	release, err := LockAssetTree(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	assetLockTryProcess(t, root, "try-reader", "BUSY")
	assetLockTryProcess(t, root, "try-writer", "BUSY")
	release()
}

func TestAssetTreeReleasedOnProcessExitAndPreCancelled(t *testing.T) {
	root := t.TempDir()
	cmd, _ := assetLockHoldProcess(t, root, "hold-writer")
	assetLockTryProcess(t, root, "try-reader", "BUSY")
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	release, err := LockAssetTree(context.Background(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if release, err := LockAssetTree(ctx, root, false); !errors.Is(err, context.Canceled) {
		if release != nil {
			release()
		}
		t.Fatalf("pre-cancelled lock operation: %v", err)
	}
}

func TestAssetLockProcessHelper(t *testing.T) {
	mode, root := os.Getenv("HB_ASSET_LOCK_HELPER"), os.Getenv("HB_ASSET_LOCK_ROOT")
	if mode == "" {
		return
	}
	var unlock func()
	var err error
	if strings.HasSuffix(mode, "session") {
		unlock, err = LockAssetSession(root, "test")
		if strings.HasPrefix(mode, "try-") && err != nil {
			if !strings.Contains(err.Error(), "outputs are in use by another operation") {
				t.Fatal(err)
			}
			fmt.Println("BUSY")
			return
		}
	} else if strings.HasPrefix(mode, "try-") {
		path, pathErr := assetLockPath(root, "tree")
		if pathErr != nil {
			t.Fatal(pathErr)
		}
		var busy bool
		unlock, busy, err = tryLockAssetTree(path, mode == "try-reader")
		if err == nil && busy {
			fmt.Println("BUSY")
			return
		}
	} else {
		unlock, err = LockAssetTree(context.Background(), root, mode == "hold-reader")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if strings.HasPrefix(mode, "try-") {
		fmt.Println("ACQUIRED")
		return
	}
	fmt.Println("READY")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
