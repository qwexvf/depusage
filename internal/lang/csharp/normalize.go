package csharp

// DepKey is intentionally empty for C#. NuGet packages don't map 1:1
// to namespaces (e.g. `Microsoft.Extensions.Logging` is split across
// many packages), so resolution requires consumer-side metadata.
func DepKey(_ string) string {
	return ""
}
