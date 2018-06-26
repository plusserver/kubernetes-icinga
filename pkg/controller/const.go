package main

const (

	// This Icinga2 Var contains the cluster name.
	VarCluster = "kubernetes_cluster"

	// The object type to monitor.
	VarType = "kubernetes_type"

	// The object name to monitor.
	VarName = "kubernetes_name"

	// The namespace containing the monitored object.
	VarNamespace = "kubernetes_namespace"

	// Namespace/Name of the custom resource that owns this icinga object
	VarOwner = "kubernetes_owner"

	// Disable monitoring
	AnnDisableMonitoring = "icinga.nexinto.com/nomonitoring"

	// Notes
	AnnNotes = "icinga.nexinto.com/notes"

	// NotesURL
	AnnNotesURL = "icinga.nexinto.com/notesurl"

	EMPTY = "<EMPTY>"
)
