//go:build darwin
// +build darwin

package platform

/*
#cgo LDFLAGS: -framework Security -framework Foundation

#include <stdlib.h>
#include <Security/Security.h>
#include <stdio.h>
#include <unistd.h>

// We use AuthorizationExecuteWithPrivileges (deprecated but still supported) to prompt for password and run sing-box for TUN.
// A single AuthorizationRef is kept and reused so the user is prompted for password only once per app session.
// If the child prints decimal PIDs on the first two lines of stdout (script PID, then sing-box PID), they are set; otherwise 0.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static AuthorizationRef g_privilegedAuthRef = NULL;

static int runWithPrivileges(const char *path, char **args, int argCount, pid_t *outScriptPid, pid_t *outSingboxPid) {
	*outScriptPid = 0;
	*outSingboxPid = 0;
	if (g_privilegedAuthRef == NULL) {
		OSStatus status = AuthorizationCreate(NULL, kAuthorizationEmptyEnvironment,
			kAuthorizationFlagInteractionAllowed | kAuthorizationFlagExtendRights,
			&g_privilegedAuthRef);
		if (status != errAuthorizationSuccess) {
			return (int)status;
		}
	}

	FILE *pipe = NULL;
	OSStatus status = AuthorizationExecuteWithPrivileges(g_privilegedAuthRef, path,
		kAuthorizationFlagDefaults, args, &pipe);
	// Do not free g_privilegedAuthRef here; reuse for next RunWithPrivileges

	if (status != errAuthorizationSuccess) {
		return (int)status;
	}
	if (pipe) {
		char buf[32];
		if (fgets(buf, (int)sizeof(buf), pipe)) {
			long p = strtol(buf, NULL, 10);
			if (p > 0)
				*outScriptPid = (pid_t)p;
		}
		if (fgets(buf, (int)sizeof(buf), pipe)) {
			long p = strtol(buf, NULL, 10);
			if (p > 0)
				*outSingboxPid = (pid_t)p;
		}
		fclose(pipe);
	}
	return 0;
}

void freePrivilegedAuthorization(void) {
	if (g_privilegedAuthRef != NULL) {
		AuthorizationFree(g_privilegedAuthRef, kAuthorizationFlagDestroyRights);
		g_privilegedAuthRef = NULL;
	}
}
#pragma clang diagnostic pop
*/
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"
)

// RunWithPrivileges runs the given tool with elevated privileges using the macOS
// Security framework. The user is prompted for their password. It returns as soon
// as the child is started; if the child prints two decimal PIDs on the first two
// lines of stdout (script PID, then sing-box PID), they are returned. Otherwise 0, 0.
// Used to start sing-box with TUN or to kill the privileged process.
func RunWithPrivileges(toolPath string, args []string) (scriptPID, singboxPID int, err error) {
	cPath := C.CString(toolPath)
	defer C.free(unsafe.Pointer(cPath))

	// Build NULL-terminated array of C strings for arguments
	cArgs := make([]*C.char, 0, len(args)+1)
	for _, a := range args {
		cArgs = append(cArgs, C.CString(a))
	}
	defer func() {
		for _, p := range cArgs {
			C.free(unsafe.Pointer(p))
		}
	}()
	// NULL terminator
	cArgs = append(cArgs, nil)
	cArgsPtr := &cArgs[0]

	var cScriptPid, cSingboxPid C.pid_t
	code := C.runWithPrivileges(cPath, cArgsPtr, C.int(len(args)), &cScriptPid, &cSingboxPid)
	if code != 0 {
		return 0, 0, fmt.Errorf("privileged execution failed with status %d (authorization may have been cancelled)", code)
	}
	return int(cScriptPid), int(cSingboxPid), nil
}

// WaitForPrivilegedExit waits for the process pid to exit (reaps it to avoid zombie). Darwin only.
func WaitForPrivilegedExit(pid int) {
	if pid <= 0 {
		return
	}
	var status syscall.WaitStatus
	_, _ = syscall.Wait4(pid, &status, 0, nil)
}

// FreePrivilegedAuthorization releases the cached AuthorizationRef so the next RunWithPrivileges will prompt again.
// Call on app exit (e.g. GracefulExit) to avoid leaving the ref alive.
func FreePrivilegedAuthorization() {
	C.freePrivilegedAuthorization()
}
