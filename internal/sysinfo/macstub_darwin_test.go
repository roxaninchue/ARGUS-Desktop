//go:build darwin

package sysinfo

import (
	"os/exec"
	"testing"
)

// The /usr/bin developer stubs are skipped exactly when running them would pop Apple's
// "install the command line developer tools" dialog.
func TestMacStubMatchesXcodeSelect(t *testing.T) {
	cltMissing := exec.Command("/usr/bin/xcode-select", "-p").Run() != nil
	for _, p := range []string{"/usr/bin/git", "/usr/bin/make", "/usr/bin/python3"} {
		if got := MacStub(p); got != cltMissing {
			t.Fatalf("MacStub(%s) = %v, want %v", p, got, cltMissing)
		}
	}
	jdkMissing := exec.Command("/usr/libexec/java_home").Run() != nil
	if got := MacStub("/usr/bin/java"); got != jdkMissing {
		t.Fatalf("MacStub(java) = %v, want %v", got, jdkMissing)
	}
	if MacStub("/bin/ls") || MacStub("/usr/bin/sw_vers") {
		t.Fatal("ordinary programs are never stubs")
	}
}
