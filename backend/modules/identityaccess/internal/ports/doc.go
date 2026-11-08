// Package ports declares the seams the Identity & Access application needs,
// one file per kind: the stores of its own tables (persistence, records),
// the unit of work (runtime), the rules it consults (policy) and the ports
// other owners bind (owners), beside the ports of single flows. Composition
// binds them.
package ports
