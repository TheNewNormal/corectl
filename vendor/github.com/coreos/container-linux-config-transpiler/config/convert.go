// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"net/url"
	"reflect"

	"github.com/alecthomas/units"
	"github.com/coreos/container-linux-config-transpiler/config/types"
	ignTypes "github.com/coreos/ignition/config/types"
	"github.com/coreos/ignition/config/validate"
	"github.com/coreos/ignition/config/validate/report"
	"github.com/vincent-petithory/dataurl"
)

const (
	BYTES_PER_SECTOR = 512
)

func ConvertAs2_0_0(in types.Config) (ignTypes.Config, report.Report) {
	out := ignTypes.Config{
		Ignition: ignTypes.Ignition{
			Version: ignTypes.IgnitionVersion{Major: 2, Minor: 0},
		},
	}

	for _, ref := range in.Ignition.Config.Append {
		newRef, err := convertConfigReference(ref)
		if err != nil {
			return ignTypes.Config{}, report.ReportFromError(err, report.EntryError)
		}
		out.Ignition.Config.Append = append(out.Ignition.Config.Append, newRef)
	}

	if in.Ignition.Config.Replace != nil {
		newRef, err := convertConfigReference(*in.Ignition.Config.Replace)
		if err != nil {
			return ignTypes.Config{}, report.ReportFromError(err, report.EntryError)
		}
		out.Ignition.Config.Replace = &newRef
	}

	for _, disk := range in.Storage.Disks {
		newDisk := ignTypes.Disk{
			Device:    ignTypes.Path(disk.Device),
			WipeTable: disk.WipeTable,
		}

		for _, partition := range disk.Partitions {
			size, err := convertPartitionDimension(partition.Size)
			if err != nil {
				return ignTypes.Config{}, report.ReportFromError(err, report.EntryError)
			}
			start, err := convertPartitionDimension(partition.Start)
			if err != nil {
				return ignTypes.Config{}, report.ReportFromError(err, report.EntryError)
			}

			newDisk.Partitions = append(newDisk.Partitions, ignTypes.Partition{
				Label:    ignTypes.PartitionLabel(partition.Label),
				Number:   partition.Number,
				Size:     size,
				Start:    start,
				TypeGUID: ignTypes.PartitionTypeGUID(partition.TypeGUID),
			})
		}

		out.Storage.Disks = append(out.Storage.Disks, newDisk)
	}

	for _, array := range in.Storage.Arrays {
		newArray := ignTypes.Raid{
			Name:   array.Name,
			Level:  array.Level,
			Spares: array.Spares,
		}

		for _, device := range array.Devices {
			newArray.Devices = append(newArray.Devices, ignTypes.Path(device))
		}

		out.Storage.Arrays = append(out.Storage.Arrays, newArray)
	}

	for _, filesystem := range in.Storage.Filesystems {
		newFilesystem := ignTypes.Filesystem{
			Name: filesystem.Name,
			Path: func(p ignTypes.Path) *ignTypes.Path {
				if p == "" {
					return nil
				}

				return &p
			}(ignTypes.Path(filesystem.Path)),
		}

		if filesystem.Mount != nil {
			newFilesystem.Mount = &ignTypes.FilesystemMount{
				Device: ignTypes.Path(filesystem.Mount.Device),
				Format: ignTypes.FilesystemFormat(filesystem.Mount.Format),
			}

			if filesystem.Mount.Create != nil {
				newFilesystem.Mount.Create = &ignTypes.FilesystemCreate{
					Force:   filesystem.Mount.Create.Force,
					Options: ignTypes.MkfsOptions(filesystem.Mount.Create.Options),
				}
			}
		}

		out.Storage.Filesystems = append(out.Storage.Filesystems, newFilesystem)
	}

	for _, file := range in.Storage.Files {
		newFile := ignTypes.File{
			Filesystem: file.Filesystem,
			Path:       ignTypes.Path(file.Path),
			Mode:       ignTypes.FileMode(file.Mode),
			User:       ignTypes.FileUser{Id: file.User.Id},
			Group:      ignTypes.FileGroup{Id: file.Group.Id},
		}

		if file.Contents.Inline != "" {
			newFile.Contents = ignTypes.FileContents{
				Source: ignTypes.Url{
					Scheme: "data",
					Opaque: "," + dataurl.EscapeString(file.Contents.Inline),
				},
			}
		}

		if file.Contents.Remote.Url != "" {
			source, err := url.Parse(file.Contents.Remote.Url)
			if err != nil {
				return ignTypes.Config{}, report.ReportFromError(err, report.EntryError)
			}

			newFile.Contents = ignTypes.FileContents{Source: ignTypes.Url(*source)}
		}

		if newFile.Contents == (ignTypes.FileContents{}) {
			newFile.Contents = ignTypes.FileContents{
				Source: ignTypes.Url{
					Scheme: "data",
					Opaque: ",",
				},
			}
		}

		newFile.Contents.Compression = ignTypes.Compression(file.Contents.Remote.Compression)
		newFile.Contents.Verification = convertVerification(file.Contents.Remote.Verification)

		out.Storage.Files = append(out.Storage.Files, newFile)
	}

	for _, unit := range in.Systemd.Units {
		newUnit := ignTypes.SystemdUnit{
			Name:     ignTypes.SystemdUnitName(unit.Name),
			Enable:   unit.Enable,
			Mask:     unit.Mask,
			Contents: unit.Contents,
		}

		for _, dropIn := range unit.DropIns {
			newUnit.DropIns = append(newUnit.DropIns, ignTypes.SystemdUnitDropIn{
				Name:     ignTypes.SystemdUnitDropInName(dropIn.Name),
				Contents: dropIn.Contents,
			})
		}

		out.Systemd.Units = append(out.Systemd.Units, newUnit)
	}

	for _, unit := range in.Networkd.Units {
		out.Networkd.Units = append(out.Networkd.Units, ignTypes.NetworkdUnit{
			Name:     ignTypes.NetworkdUnitName(unit.Name),
			Contents: unit.Contents,
		})
	}

	for _, user := range in.Passwd.Users {
		newUser := ignTypes.User{
			Name:              user.Name,
			PasswordHash:      user.PasswordHash,
			SSHAuthorizedKeys: user.SSHAuthorizedKeys,
		}

		if user.Create != nil {
			newUser.Create = &ignTypes.UserCreate{
				Uid:          user.Create.Uid,
				GECOS:        user.Create.GECOS,
				Homedir:      user.Create.Homedir,
				NoCreateHome: user.Create.NoCreateHome,
				PrimaryGroup: user.Create.PrimaryGroup,
				Groups:       user.Create.Groups,
				NoUserGroup:  user.Create.NoUserGroup,
				System:       user.Create.System,
				NoLogInit:    user.Create.NoLogInit,
				Shell:        user.Create.Shell,
			}
		}

		out.Passwd.Users = append(out.Passwd.Users, newUser)
	}

	for _, group := range in.Passwd.Groups {
		out.Passwd.Groups = append(out.Passwd.Groups, ignTypes.Group{
			Name:         group.Name,
			Gid:          group.Gid,
			PasswordHash: group.PasswordHash,
			System:       group.System,
		})
	}

	r := validate.ValidateWithoutSource(reflect.ValueOf(out))
	if r.IsFatal() {
		return ignTypes.Config{}, r
	}

	return out, r
}

func convertConfigReference(in types.ConfigReference) (ignTypes.ConfigReference, error) {
	source, err := url.Parse(in.Source)
	if err != nil {
		return ignTypes.ConfigReference{}, err
	}

	return ignTypes.ConfigReference{
		Source:       ignTypes.Url(*source),
		Verification: convertVerification(in.Verification),
	}, nil
}

func convertVerification(in types.Verification) ignTypes.Verification {
	if in.Hash.Function == "" || in.Hash.Sum == "" {
		return ignTypes.Verification{}
	}

	return ignTypes.Verification{
		&ignTypes.Hash{
			Function: in.Hash.Function,
			Sum:      in.Hash.Sum,
		},
	}
}

func convertPartitionDimension(in string) (ignTypes.PartitionDimension, error) {
	if in == "" {
		return 0, nil
	}

	b, err := units.ParseBase2Bytes(in)
	if err != nil {
		return 0, err
	}
	if b < 0 {
		return 0, fmt.Errorf("invalid dimension (negative): %q", in)
	}

	// Translate bytes into sectors
	sectors := (b / BYTES_PER_SECTOR)
	if b%BYTES_PER_SECTOR != 0 {
		sectors++
	}
	return ignTypes.PartitionDimension(uint64(sectors)), nil
}
