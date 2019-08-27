package iso

import (
	"context"
	"fmt"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/jetbrains-infra/packer-builder-vsphere/common"
	"github.com/jetbrains-infra/packer-builder-vsphere/driver"
)

type CreateConfig struct {
	Version     uint   `mapstructure:"vm_version"`
	GuestOSType string `mapstructure:"guest_os_type"`
	Firmware    string `mapstructure:"firmware"`

	DiskControllerType  string `mapstructure:"disk_controller_type"`
	DiskSize            int64  `mapstructure:"disk_size"`
	DiskThinProvisioned bool   `mapstructure:"disk_thin_provisioned"`

	Network       string `mapstructure:"network"`
	NetworkCard   string `mapstructure:"network_card"`
	USBController bool   `mapstructure:"usb_controller"`

	Notes string `mapstructure:"notes"`

	GlobalDiskType string              `mapstructure:"disk_type"`
	Networks       []string            `mapstructure:"networks"`
	Storage        []driver.DiskConfig `mapstructure:"storage"`
}

func (c *CreateConfig) Prepare() []error {
	var errs []error

	// options 'DiskSize' and 'Storage' are but mutually exclusive, but one is required
	if c.DiskSize != 0 && len(c.Storage) != 0 {
		errs = append(errs, fmt.Errorf("'disk_size' and 'storage' are mutually exclusive"))
	} else if c.DiskSize == 0 && len(c.Storage) == 0 {
		errs = append(errs, fmt.Errorf("either 'disk_size' or 'storage' is required"))
	}

	if c.GuestOSType == "" {
		c.GuestOSType = "otherGuest"
	}

	if c.Firmware != "" && c.Firmware != "bios" && c.Firmware != "efi" {
		errs = append(errs, fmt.Errorf("'firmware' must be 'bios' or 'efi'"))
	}

	return errs
}

type StepCreateVM struct {
	Config   *CreateConfig
	Location *common.LocationConfig
	Force    bool
}

func (s *StepCreateVM) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(*driver.Driver)

	vm, err := d.FindVM(s.Location.VMName)

	if s.Force == false && err == nil {
		state.Put("error", fmt.Errorf("%s already exists, you can use -force flag to destroy it: %v", s.Location.VMName, err))
		return multistep.ActionHalt
	} else if s.Force == true && err == nil {
		ui.Say(fmt.Sprintf("the vm/template %s already exists, but deleting it due to -force flag", s.Location.VMName))
		err := vm.Destroy()
		if err != nil {
			state.Put("error", fmt.Errorf("error destroying %s: %v", s.Location.VMName, err))
		}
	}

	ui.Say("Creating VM...")
	vm, err = d.CreateVM(&driver.CreateConfig{
		DiskThinProvisioned: s.Config.DiskThinProvisioned,
		DiskControllerType:  s.Config.DiskControllerType,
		DiskSize:            s.Config.DiskSize,
		Name:                s.Location.VMName,
		Folder:              s.Location.Folder,
		Cluster:             s.Location.Cluster,
		Host:                s.Location.Host,
		ResourcePool:        s.Location.ResourcePool,
		Datastore:           s.Location.Datastore,
		GuestOS:             s.Config.GuestOSType,
		Network:             s.Config.Network,
		NetworkCard:         s.Config.NetworkCard,
		USBController:       s.Config.USBController,
		Version:             s.Config.Version,
		Firmware:            s.Config.Firmware,
		Annotation:          s.Config.Notes,
		GlobalDiskType:      s.Config.GlobalDiskType,
		Networks:            s.Config.Networks,
		Storage:             s.Config.Storage,
	})
	if err != nil {
		state.Put("error", fmt.Errorf("error creating vm: %v", err))
		return multistep.ActionHalt
	}
	state.Put("vm", vm)

	return multistep.ActionContinue
}

func (s *StepCreateVM) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	ui := state.Get("ui").(packer.Ui)

	st := state.Get("vm")
	if st == nil {
		return
	}
	vm := st.(*driver.VirtualMachine)

	ui.Say("Destroying VM...")
	err := vm.Destroy()
	if err != nil {
		ui.Error(err.Error())
	}
}
