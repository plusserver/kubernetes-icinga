package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HostGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostGroupSpec   `json:"spec"`
	Status HostGroupStatus `json:"status"`
}

type HostGroupSpec struct {
	Name string            `json:"name"`
	Vars map[string]string `json:"vars"`
}

type HostGroupStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HostGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HostGroup `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Host struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostSpec   `json:"spec"`
	Status HostStatus `json:"status"`
}

type HostSpec struct {
	Name         string            `json:"name"`
	Vars         map[string]string `json:"vars"`
	Hostgroups   []string          `json:"hostgroups"`
	CheckCommand string            `json:"check_command,omitempty"`
	Notes        string            `json:"notes"`
	NotesURL     string            `json:"notesurl"`
}

type HostStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Host `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Check struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckSpec   `json:"spec"`
	Status CheckStatus `json:"status"`
}

type CheckSpec struct {
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	CheckCommand string            `json:"checkcommand"`
	Notes        string            `json:"notes"`
	NotesURL     string            `json:"notesurl"`
	Vars         map[string]string `json:"vars"`
}

type CheckStatus struct {
	Synced bool `json:"synced,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Check `json:"items"`
}
