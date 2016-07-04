// Copyright (c) 2016 by António Meireles  <antonio.meireles@reformi.st>.
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
//

package connector

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"github.com/rakyll/pb"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type SSHclient struct {
	session                   *ssh.Session
	conn                      *ssh.Client
	oldState                  *terminal.State
	termWidth, termHeight, fd int
}

func (c *SSHclient) Close() {
	c.conn.Close()
	c.session.Close()
	terminal.Restore(c.fd, c.oldState)
}
func (c *SSHclient) ExecuteRemoteCommand(run string) (err error) {
	if err = c.session.Run(run); err != nil && !strings.HasSuffix(err.Error(),
		"exited without exit status or exit signal") {
		return
	}
	return nil
}

func (c *SSHclient) RemoteShell() (err error) {
	if err = c.session.Shell(); err != nil {
		return
	}

	if err = c.session.Wait(); err != nil && !strings.HasSuffix(err.Error(),
		"exited without exit status or exit signal") {
		return err
	}
	return nil
}

func (c *SSHclient) SCoPy(source, destination, target string) (err error) {
	var (
		ftp         *sftp.Client
		src         *os.File
		srcS, destS os.FileInfo
		dest        *sftp.File
		bar         *pb.ProgressBar
	)

	if ftp, err = sftp.NewClient(c.conn); err != nil {
		return
	}
	defer ftp.Close()

	if src, err = os.Open(source); err != nil {
		return
	}
	defer src.Close()
	if srcS, err = os.Stat(source); err != nil {
		return
	}
	if _, err = ftp.ReadDir(filepath.Dir(destination)); err != nil {
		err = fmt.Errorf("unable to upload %v as parent %v "+
			"not in target", source, filepath.Dir(destination))
		return
	}
	if _, err = ftp.ReadDir(destination); err == nil {
		destination = ftp.Join(destination, "/", filepath.Base(source))
	}
	if dest, err = ftp.Create(destination); err != nil {
		return
	}
	defer dest.Close()
	log.Println("uploading '" + source + "' to '" +
		target + ":" + destination + "'")
	bar = pb.New(int(srcS.Size())).SetUnits(pb.U_BYTES)
	bar.Start()
	writer := io.MultiWriter(bar, dest)
	defer bar.Finish()
	if _, err = io.Copy(writer, src); err != nil {
		return
	}

	if destS, err = ftp.Stat(destination); err != nil {
		return
	}
	if srcS.Size() != destS.Size() {
		err = fmt.Errorf("something went wrong. " +
			"destination file size != from sources'")
	}
	return
}

func StartSSHsession(ip string, privateKey string) (c *SSHclient, err error) {
	var secret ssh.Signer
	c = &SSHclient{}

	if secret, err = ssh.ParsePrivateKey(
		[]byte(privateKey)); err != nil {
		return
	}

	config := &ssh.ClientConfig{
		User: "core", Auth: []ssh.AuthMethod{
			ssh.PublicKeys(secret),
		},
	}

	if c.conn, err = ssh.Dial("tcp", ip+":22", config); err != nil {
		return c, fmt.Errorf("%s unreachable", ip+":22")
	}

	if c.session, err = c.conn.NewSession(); err != nil {
		return c, fmt.Errorf("unable to create session: %s", err)
	}

	c.fd = int(os.Stdin.Fd())
	if c.oldState, err = terminal.MakeRaw(c.fd); err != nil {
		return
	}

	c.session.Stdout, c.session.Stderr, c.session.Stdin =
		os.Stdout, os.Stderr, os.Stdin

	if c.termWidth, c.termHeight, err = terminal.GetSize(c.fd); err != nil {
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400,
	}

	if err = c.session.RequestPty("xterm-256color",
		c.termHeight, c.termWidth, modes); err != nil {
		return c, fmt.Errorf("request for pseudo terminal failed: %s", err)
	}
	return
}
