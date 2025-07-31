package main

// Clone implementations for types that need to be Cloner[T].

// Clone creates a deep copy of FleetEvent.
func (e FleetEvent) Clone() FleetEvent {
	clone := e

	// Deep copy maps.
	if e.Data != nil {
		clone.Data = make(map[string]any, len(e.Data))
		for k, v := range e.Data {
			clone.Data[k] = v
		}
	}

	if e.DomainResults != nil {
		clone.DomainResults = make(map[string]any, len(e.DomainResults))
		for k, v := range e.DomainResults {
			clone.DomainResults[k] = v
		}
	}

	return clone
}
