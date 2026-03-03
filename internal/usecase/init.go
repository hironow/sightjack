package usecase

import (
	"io/fs"

	"github.com/hironow/sightjack/internal/session"
)

// WriteGitIgnore writes the .gitignore inside .siren/.
func WriteGitIgnore(baseDir string) error {
	return session.WriteGitIgnore(baseDir)
}

// InstallSkills copies embedded skill files to the project.
func InstallSkills(baseDir string, skillsFS fs.FS) error {
	return session.InstallSkills(baseDir, skillsFS)
}

// EnsureMailDirs creates inbox/, outbox/, archive/ under .siren/.
func EnsureMailDirs(baseDir string) error {
	return session.EnsureMailDirs(baseDir)
}
