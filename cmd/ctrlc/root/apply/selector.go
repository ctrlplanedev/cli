package apply

import "github.com/ctrlplanedev/cli/internal/api/providers"

func applySelectorToSpecs(selector *providers.Selector, specs []providers.TypedSpec) {
	if selector == nil {
		return
	}

	for _, spec := range specs {
		switch typed := spec.Spec.(type) {
		case *providers.ResourceItemSpec:
			typed.Metadata = selector.ApplyMetadata(typed.Metadata)
		}
	}
}
