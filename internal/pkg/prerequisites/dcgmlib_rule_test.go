/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prerequisites

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	debugelf "debug/elf"

	"github.com/stretchr/testify/require"

	mockelf "github.com/NVIDIA/dcgm-exporter/internal/mocks/pkg/elf"
	mockexec "github.com/NVIDIA/dcgm-exporter/internal/mocks/pkg/exec"
)

func Test_dcgmLibExistsRule_Validate(t *testing.T) {
	ldconfigPath := "/sbin/ldconfig.real"

	type testCase struct {
		Name                 string
		ExecMockExpectations func(*gomock.Controller, *mockexec.MockExec)
		ELFMockExpectations  func(*gomock.Controller, *mockelf.MockELF)
		AssertErr            func(err error)
	}

	testCases := []testCase{
		{
			Name: "no error",
			ExecMockExpectations: func(ctrl *gomock.Controller, mockExec *mockexec.MockExec) {
				output := `1211 libs found in cache '/etc/ld.so.cache'
				libdcgm.so.4 (libc6,x86-64) => /lib/x86_64-linux-gnu/libdcgm.so.4
			Cache generated by: ldconfig (Ubuntu GLIBC 2.35-0ubuntu3.7) stable release version 2.35`
				cmd := mockexec.NewMockCmd(ctrl)
				cmd.EXPECT().Output().AnyTimes().Return([]byte(output), nil)
				mockExec.EXPECT().Command(gomock.Eq(ldconfigPath), gomock.Eq(ldconfigParam)).AnyTimes().Return(cmd)
			},
			ELFMockExpectations: func(c *gomock.Controller, mockELF *mockelf.MockELF) {
				self := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_X86_64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/proc/self/exe")).AnyTimes().Return(self, nil)

				libdcgm := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_X86_64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/lib/x86_64-linux-gnu/libdcgm.so.4")).AnyTimes().Return(libdcgm, nil)
			},
			AssertErr: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "returns error when library is not found",
			ExecMockExpectations: func(ctrl *gomock.Controller, mockExec *mockexec.MockExec) {
				output := `1211 libs found in cache '/etc/ld.so.cache'
				libcuda.so (libc6,x86-64) => /lib/x86_64-linux-gnu/libcuda.so
			Cache generated by: ldconfig (Ubuntu GLIBC 2.35-0ubuntu3.7) stable release version 2.35`
				cmd := mockexec.NewMockCmd(ctrl)
				cmd.EXPECT().Output().AnyTimes().Return([]byte(output), nil)
				mockExec.EXPECT().Command(gomock.Eq(ldconfigPath), gomock.Eq(ldconfigParam)).AnyTimes().Return(cmd)
			},
			ELFMockExpectations: func(c *gomock.Controller, mockELF *mockelf.MockELF) {
				self := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_X86_64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/proc/self/exe")).AnyTimes().Return(self, nil)

				libdcgm := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_X86_64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/lib/x86_64-linux-gnu/libdcgm.so.4")).AnyTimes().Return(libdcgm, nil)
			},
			AssertErr: func(err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "the libdcgm.so.4 library was not found. Install Data Center GPU Manager (DCGM).")
			},
		},
		{
			Name: "returns error when can not execute command",
			ExecMockExpectations: func(ctrl *gomock.Controller, mockExec *mockexec.MockExec) {
				cmd := mockexec.NewMockCmd(ctrl)
				cmd.EXPECT().Output().AnyTimes().Return([]byte{}, errors.New("boom!"))
				mockExec.EXPECT().Command(gomock.Eq(ldconfigPath), gomock.Eq(ldconfigParam)).AnyTimes().Return(cmd)
			},
			AssertErr: func(err error) {
				require.Error(t, err)
			},
		},
		{
			Name: "error when can not open /proc/self/exe",
			ExecMockExpectations: func(ctrl *gomock.Controller, mockExec *mockexec.MockExec) {
				output := `1211 libs found in cache '/etc/ld.so.cache'
				libdcgm.so.4 (libc6,x86-64) => /lib/x86_64-linux-gnu/libdcgm.so.4
			Cache generated by: ldconfig (Ubuntu GLIBC 2.35-0ubuntu3.7) stable release version 2.35`
				cmd := mockexec.NewMockCmd(ctrl)
				cmd.EXPECT().Output().AnyTimes().Return([]byte(output), nil)
				mockExec.EXPECT().Command(gomock.Eq(ldconfigPath), gomock.Eq(ldconfigParam)).AnyTimes().Return(cmd)
			},
			ELFMockExpectations: func(c *gomock.Controller, mockELF *mockelf.MockELF) {
				mockELF.EXPECT().Open(gomock.Eq("/proc/self/exe")).AnyTimes().Return(nil, errors.New("boom!"))
			},
			AssertErr: func(err error) {
				require.Error(t, err)
			},
		},
		{
			Name: "returns error when library architecture missmatch",
			ExecMockExpectations: func(ctrl *gomock.Controller, mockExec *mockexec.MockExec) {
				output := `1211 libs found in cache '/etc/ld.so.cache'
				libdcgm.so.4 (libc6,x86-64) => /lib/x86_64-linux-gnu/libdcgm.so.4
			Cache generated by: ldconfig (Ubuntu GLIBC 2.35-0ubuntu3.7) stable release version 2.35`
				cmd := mockexec.NewMockCmd(ctrl)
				cmd.EXPECT().Output().AnyTimes().Return([]byte(output), nil)
				mockExec.EXPECT().Command(gomock.Eq(ldconfigPath), gomock.Eq(ldconfigParam)).AnyTimes().Return(cmd)
			},
			ELFMockExpectations: func(c *gomock.Controller, mockELF *mockelf.MockELF) {
				self := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_X86_64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/proc/self/exe")).AnyTimes().Return(self, nil)

				libdcgm := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_AARCH64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/lib/x86_64-linux-gnu/libdcgm.so.4")).AnyTimes().Return(libdcgm, nil)
			},
			AssertErr: func(err error) {
				require.Error(t, err)
				require.ErrorContains(t, err,
					"the libdcgm.so.4 library architecture mismatch with the system; wanted: EM_X86_64, received: EM_AARCH64")
			},
		},
		{
			Name: "returns error when library file can not be open",
			ExecMockExpectations: func(ctrl *gomock.Controller, mockExec *mockexec.MockExec) {
				output := `1211 libs found in cache '/etc/ld.so.cache'
				libdcgm.so.4 (libc6,x86-64) => /lib/x86_64-linux-gnu/libdcgm.so.4
			Cache generated by: ldconfig (Ubuntu GLIBC 2.35-0ubuntu3.7) stable release version 2.35`
				cmd := mockexec.NewMockCmd(ctrl)
				cmd.EXPECT().Output().AnyTimes().Return([]byte(output), nil)
				mockExec.EXPECT().Command(gomock.Eq(ldconfigPath), gomock.Eq(ldconfigParam)).AnyTimes().Return(cmd)
			},
			ELFMockExpectations: func(c *gomock.Controller, mockELF *mockelf.MockELF) {
				self := &debugelf.File{
					FileHeader: debugelf.FileHeader{
						Machine: debugelf.EM_X86_64,
					},
				}
				mockELF.EXPECT().Open(gomock.Eq("/proc/self/exe")).AnyTimes().Return(self, nil)

				mockELF.EXPECT().Open(gomock.Eq("/lib/x86_64-linux-gnu/libdcgm.so.4")).AnyTimes().Return(nil, errors.New("boom!"))
			},
			AssertErr: func(err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			executor := mockexec.NewMockExec(ctrl)

			if tc.ExecMockExpectations != nil {
				tc.ExecMockExpectations(ctrl, executor)
			}
			exec = executor

			elfreader := mockelf.NewMockELF(ctrl)

			if tc.ELFMockExpectations != nil {
				tc.ELFMockExpectations(ctrl, elfreader)
			}
			elf = elfreader

			err := dcgmLibExistsRule{}.Validate()
			tc.AssertErr(err)
		})
	}
}
