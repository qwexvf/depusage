package php

// DepKey is intentionally empty for PHP. Composer maps namespaces to
// packages via composer.json autoload sections — depusage can't
// resolve that without project-side data. Consumers (aegis, SBOM
// tooling) match Module against composer.lock metadata.
func DepKey(namespace string) string {
	return ""
}
