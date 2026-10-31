package site

import (
	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
	"github.com/mutms/mdl-demo/go/internal/state"
)

// applyCodeAccess hands the tree to www-data in a demo created with
// MDL_DEMO_EDITABLE_CODE. Deferred by every job that writes to the tree: they run
// as root, and would otherwise leave files and .git objects www-data can't change.
func applyCodeAccess(logf execx.Logf) {
	if !state.CodeEditable() {
		return
	}
	if err := execx.Run(logf, "", "chown", "-R", "www-data:www-data", moodle.Root); err != nil {
		logf("warning: making the source code editable failed: " + err.Error())
	}
}
