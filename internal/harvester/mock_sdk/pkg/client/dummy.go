package client

import (
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// --- Mock Harvester SDK types (simplified) ---

// VirtualMachine represents a Harvester virtual machine.
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              VirtualMachineSpec   `json:"spec,omitempty"`
	Status            VirtualMachineStatus `json:"status,omitempty"`
}

// DeepCopy creates a deep copy of the VirtualMachine.
// This is a manual implementation for the mock.
func (in *VirtualMachine) DeepCopy() *VirtualMachine {
	if in == nil {
		return nil
	}
	out := new(VirtualMachine)
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = *in.ObjectMeta.DeepCopy() // metav1.ObjectMeta has a DeepCopy

	// Deep copy Spec
	out.Spec.RunStrategy = in.Spec.RunStrategy
	if in.Spec.Running != nil {
		b := *in.Spec.Running
		out.Spec.Running = &b
	}
	out.Spec.Template.Spec.CPU = in.Spec.Template.Spec.CPU
	out.Spec.Template.Spec.Memory = in.Spec.Template.Spec.Memory
	out.Spec.Template.Spec.UserData = in.Spec.Template.Spec.UserData
	if in.Spec.Template.Spec.Networks != nil {
		out.Spec.Template.Spec.Networks = make([]NetworkConfig, len(in.Spec.Template.Spec.Networks))
		for i, nw := range in.Spec.Template.Spec.Networks {
			out.Spec.Template.Spec.Networks[i] = nw // NetworkConfig is simple struct
		}
	}
	if in.Spec.Template.Spec.Disks != nil {
		out.Spec.Template.Spec.Disks = make([]DiskConfig, len(in.Spec.Template.Spec.Disks))
		for i, disk := range in.Spec.Template.Spec.Disks {
			out.Spec.Template.Spec.Disks[i] = disk // DiskConfig is simple struct, careful with pointers if added
			if disk.BootOrder != nil {
				bo := *disk.BootOrder
				out.Spec.Template.Spec.Disks[i].BootOrder = &bo
			}
		}
	}

	// Deep copy Status
	out.Status.PrintableStatus = in.Status.PrintableStatus
	return out
}

// VirtualMachineList is a list of VirtualMachine objects.
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

type VirtualMachineSpec struct {
	Template    VirtualMachineTemplateSpec `json:"template"`
	RunStrategy string                     `json:"runStrategy,omitempty"`
	Running     *bool                      `json:"running,omitempty"`
}

type VirtualMachineTemplateSpec struct {
	Spec VMEmbeddedSpec `json:"spec"`
}

type VMEmbeddedSpec struct {
	CPU      int             `json:"cpu"`
	Memory   string          `json:"memory"`
	Networks []NetworkConfig `json:"networks"`
	Disks    []DiskConfig    `json:"disks"`
	UserData string          `json:"userData"`
}

type NetworkConfig struct {
	Name        string `json:"name"`
	NetworkName string `json:"networkName"`
}

type DiskConfig struct {
	Name       string `json:"name"`
	Image      string `json:"image"`
	BootOrder  *int   `json:"bootOrder,omitempty"`
	DiskBus    string `json:"diskBus"`
	VolumeMode string `json:"volumeMode"`
	Type       string `json:"type"`
	Size       string `json:"size"`
}

type VirtualMachineStatus struct {
	PrintableStatus string `json:"printableStatus"`
}

type StartOptions struct{}
type StopOptions struct{}

type HarvesterClient struct{}

func NewHarvesterClientFromConfig(cfg interface{}) (*HarvesterClient, error) {
	return &HarvesterClient{}, nil
}

type VirtualMachineInterface interface {
	Create(vm *VirtualMachine) (*VirtualMachine, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Get(name string, options metav1.GetOptions) (*VirtualMachine, error)
	List(opts metav1.ListOptions) (*VirtualMachineList, error)
	Start(name string, opts *StartOptions) error
	Stop(name string, opts *StopOptions) error
}

var mockStore = struct {
	sync.RWMutex
	vms map[string]map[string]*VirtualMachine
}{
	vms: make(map[string]map[string]*VirtualMachine),
}

// Helper to reset mockStore for tests
func ResetMockStore() {
	mockStore.Lock()
	defer mockStore.Unlock()
	mockStore.vms = make(map[string]map[string]*VirtualMachine)
}


func (c *HarvesterClient) VirtualMachines(namespace string) VirtualMachineInterface {
	fmt.Printf("MockSDK: VirtualMachines(namespace: %s) called\n", namespace)
	return &mockVirtualMachineInterface{namespace: namespace}
}

type mockVirtualMachineInterface struct {
	namespace      string
	LastCreateVM   *VirtualMachine
	LastDeleteName string
	LastGetName    string
	LastListOpts   metav1.ListOptions
	LastStartName  string
	LastStopName   string
}

func (m *mockVirtualMachineInterface) Create(vm *VirtualMachine) (*VirtualMachine, error) {
	mockStore.Lock()
	defer mockStore.Unlock()
	
	// Store a copy of the input VM for assertion by tests
	// Ensure this happens before any modification for response.
	m.LastCreateVM = vm.DeepCopy()


	fmt.Printf("MockSDK: Create VM called in namespace %s for VM: %s\n", m.namespace, vm.Name)
	if _, ok := mockStore.vms[m.namespace]; !ok {
		mockStore.vms[m.namespace] = make(map[string]*VirtualMachine)
	}

	if _, ok := mockStore.vms[m.namespace][vm.Name]; ok {
		return nil, errors.NewAlreadyExists(schema.GroupResource{Group: "harvesterhci.io", Resource: "virtualmachines"}, vm.Name)
	}

	createdVMForStore := vm.DeepCopy() // Work with a copy for storage

	if createdVMForStore.Spec.RunStrategy == "" && (createdVMForStore.Spec.Running == nil || *createdVMForStore.Spec.Running) {
		createdVMForStore.Spec.RunStrategy = "Running"
	}
	if createdVMForStore.Spec.Running == nil {
		t := true
		if createdVMForStore.Spec.RunStrategy == "Halted" {
			t = false
		}
		createdVMForStore.Spec.Running = &t
	}

	if *createdVMForStore.Spec.Running && createdVMForStore.Spec.RunStrategy == "Running" {
		createdVMForStore.Status.PrintableStatus = "Starting"
	} else {
		createdVMForStore.Status.PrintableStatus = "Stopped"
	}

	if createdVMForStore.Annotations == nil {
		createdVMForStore.Annotations = make(map[string]string)
	}
	createdVMForStore.Annotations["harvesterhci.io/vm-provider-id"] = "mock-provider-id-" + createdVMForStore.Name + "-" + time.Now().Format("20060102150405")

	mockStore.vms[m.namespace][createdVMForStore.Name] = createdVMForStore
	fmt.Printf("MockSDK: VM %s stored in mock store for namespace %s. Status: %s\n", createdVMForStore.Name, m.namespace, createdVMForStore.Status.PrintableStatus)
	
	// Return a copy of the state as it would be after creation (e.g. status might be "Provisioning" or "Starting")
	returnVMForAPI := createdVMForStore.DeepCopy()
	return returnVMForAPI, nil
}

func (m *mockVirtualMachineInterface) Delete(name string, options *metav1.DeleteOptions) error {
	mockStore.Lock()
	defer mockStore.Unlock()
	m.LastDeleteName = name
	fmt.Printf("MockSDK: Delete VM called in namespace %s for VM: %s\n", m.namespace, name)

	if nsMap, ok := mockStore.vms[m.namespace]; ok {
		if _, vmExists := nsMap[name]; vmExists {
			delete(nsMap, name)
			fmt.Printf("MockSDK: VM %s deleted from mock store in namespace %s.\n", name, m.namespace)
			return nil
		}
	}
	return errors.NewNotFound(schema.GroupResource{Group: "harvesterhci.io", Resource: "virtualmachines"}, name)
}

func (m *mockVirtualMachineInterface) Get(name string, options metav1.GetOptions) (*VirtualMachine, error) {
	mockStore.RLock()
	defer mockStore.RUnlock()
	m.LastGetName = name
	fmt.Printf("MockSDK: Get VM called in namespace %s for VM: %s\n", m.namespace, name)

	if nsMap, ok := mockStore.vms[m.namespace]; ok {
		if vm, vmExists := nsMap[name]; vmExists {
			fmt.Printf("MockSDK: VM %s found in mock store. Current status: %s\n", name, vm.Status.PrintableStatus)
			vmToReturn := vm.DeepCopy()
			if vmToReturn.Status.PrintableStatus == "Starting" {
				vmToReturn.Status.PrintableStatus = "Running"
				fmt.Printf("MockSDK: VM %s status updated to Running for Get.\n", name)
			} else if vmToReturn.Status.PrintableStatus == "Stopping" {
				vmToReturn.Status.PrintableStatus = "Stopped"
				fmt.Printf("MockSDK: VM %s status updated to Stopped for Get.\n", name)
			}
			return vmToReturn, nil
		}
	}
	fmt.Printf("MockSDK: VM %s NOT found in mock store for namespace %s.\n", name, m.namespace)
	return nil, errors.NewNotFound(schema.GroupResource{Group: "harvesterhci.io", Resource: "virtualmachines"}, name)
}

func (m *mockVirtualMachineInterface) List(opts metav1.ListOptions) (*VirtualMachineList, error) {
	mockStore.RLock()
	defer mockStore.RUnlock()
	m.LastListOpts = opts
	fmt.Printf("MockSDK: List VMs called in namespace %s with options: %+v\n", m.namespace, opts)

	var matchingVMs []VirtualMachine
	selector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("mockSDK: failed to parse label selector: %w", err)
	}

	if nsMap, ok := mockStore.vms[m.namespace]; ok {
		for _, vm := range nsMap {
			if selector.Matches(labels.Set(vm.Labels)) {
				vmCopy := vm.DeepCopy()
				if vmCopy.Status.PrintableStatus == "Starting" {
					vmCopy.Status.PrintableStatus = "Running"
				} else if vmCopy.Status.PrintableStatus == "Stopping" {
					vmCopy.Status.PrintableStatus = "Stopped"
				}
				matchingVMs = append(matchingVMs, *vmCopy)
			}
		}
	}

	fmt.Printf("MockSDK: Found %d VMs matching selector in namespace %s.\n", len(matchingVMs), m.namespace)
	return &VirtualMachineList{Items: matchingVMs}, nil
}

func (m *mockVirtualMachineInterface) Start(name string, opts *StartOptions) error {
	mockStore.Lock()
	defer mockStore.Unlock()
	m.LastStartName = name
	fmt.Printf("MockSDK: Start VM action called for %s in namespace %s\n", name, m.namespace)
	if nsMap, ok := mockStore.vms[m.namespace]; ok {
		if vm, vmExists := nsMap[name]; vmExists {
			if vm.Status.PrintableStatus == "Running" || vm.Status.PrintableStatus == "Starting" {
				fmt.Printf("MockSDK: VM %s is already %s.\n", name, vm.Status.PrintableStatus)
				return nil
			}
			fmt.Printf("MockSDK: Changing VM %s status from %s to Starting.\n", name, vm.Status.PrintableStatus)
			vm.Status.PrintableStatus = "Starting"
			vm.Spec.RunStrategy = "Running"
			t := true
			vm.Spec.Running = &t
			return nil
		}
	}
	return errors.NewNotFound(schema.GroupResource{Group: "harvesterhci.io", Resource: "virtualmachines"}, name)
}

func (m *mockVirtualMachineInterface) Stop(name string, opts *StopOptions) error {
	mockStore.Lock()
	defer mockStore.Unlock()
	m.LastStopName = name
	fmt.Printf("MockSDK: Stop VM action called for %s in namespace %s\n", name, m.namespace)
	if nsMap, ok := mockStore.vms[m.namespace]; ok {
		if vm, vmExists := nsMap[name]; vmExists {
			if vm.Status.PrintableStatus == "Stopped" || vm.Status.PrintableStatus == "Stopping" {
				fmt.Printf("MockSDK: VM %s is already %s.\n", name, vm.Status.PrintableStatus)
				return nil
			}
			fmt.Printf("MockSDK: Changing VM %s status from %s to Stopping.\n", name, vm.Status.PrintableStatus)
			vm.Status.PrintableStatus = "Stopping"
			vm.Spec.RunStrategy = "Halted"
			f := false
			vm.Spec.Running = &f
			return nil
		}
	}
	return errors.NewNotFound(schema.GroupResource{Group: "harvesterhci.io", Resource: "virtualmachines"}, name)
}
