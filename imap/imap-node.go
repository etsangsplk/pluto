package imap

import (
	"fmt"
	"io"
	stdlog "log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"crypto/tls"
	"encoding/base64"
	"io/ioutil"
	"path/filepath"

	"github.com/go-kit/kit/log"
	"github.com/numbleroot/maildir"
	"github.com/numbleroot/pluto/comm"
	"github.com/numbleroot/pluto/config"
	"github.com/numbleroot/pluto/crdt"
)

// Structs

// IMAPNode unifies needed management elements
// for nodes types worker and storage. This allows
// for one single place to define behaviour of
// handling IMAP as well as CRDT update requests.
type IMAPNode struct {
	lock             *sync.RWMutex
	logger           log.Logger
	MailSocket       net.Listener
	SyncSocket       net.Listener
	Connections      map[string]*tls.Conn
	MailboxStructure map[string]map[string]*crdt.ORSet
	MailboxContents  map[string]map[string][]string
	CRDTLayerRoot    string
	MaildirRoot      string
	Config           *config.Config
	ShutdownChan     chan struct{}
}

// Functions

// ApplyCRDTUpd receives strings representing CRDT
// update operations from receiver and executes them.
func (node *IMAPNode) ApplyCRDTUpd(applyChan chan string, doneChan chan struct{}) {

	for {

		// Receive update message from receiver
		// via channel.
		updMsg := <-applyChan

		// Parse operation that payload specifies.
		op, opPayload, err := comm.ParseOp(updMsg)
		if err != nil {
			stdlog.Fatalf("[imap.ApplyCRDTUpd] Error while parsing operation from sync message: %v", err)
		}

		// Depending on received operation,
		// parse remaining payload further.
		switch op {

		case "create":

			// Parse received payload message into create message struct.
			createUpd, err := comm.ParseCreate(opPayload)
			if err != nil {
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Error while parsing CREATE update from sync message: %v", err)
			}

			// Lock node exclusively.
			node.lock.Lock()

			// Save user's mailbox structure CRDT to more
			// conveniently use it hereafter.
			userMainCRDT := node.MailboxStructure[createUpd.User]["Structure"]

			// Create a new Maildir on stable storage.
			posMaildir := maildir.Dir(filepath.Join(node.MaildirRoot, createUpd.User, createUpd.Mailbox))

			err = posMaildir.Create()
			if err != nil {
				node.lock.Unlock()
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Maildir for new mailbox could not be created: %v", err)
			}

			// Construct path to new CRDT file.
			posMailboxCRDTPath := filepath.Join(node.CRDTLayerRoot, createUpd.User, fmt.Sprintf("%s.log", createUpd.Mailbox))

			// Initialize new ORSet for new mailbox.
			posMailboxCRDT, err := crdt.InitORSetWithFile(posMailboxCRDTPath)
			if err != nil {

				// Perform clean up.
				stdlog.Printf("[imap.ApplyCRDTUpd] CREATE fail: %v", err)
				stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created Maildir completely...\n")

				// Attempt to remove Maildir.
				err = posMaildir.Remove()
				if err != nil {
					stdlog.Printf("[imap.ApplyCRDTUpd] ... failed to remove Maildir: %v", err)
					stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
				} else {
					stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
				}

				// Exit node.
				node.lock.Unlock()
				os.Exit(1)
			}

			// Place newly created CRDT in mailbox structure.
			node.MailboxStructure[createUpd.User][createUpd.Mailbox] = posMailboxCRDT

			// Initialize contents slice for new mailbox to track
			// message sequence numbers in it.
			node.MailboxContents[createUpd.User][createUpd.Mailbox] = make([]string, 0, 6)

			// If succeeded, add a new folder in user's main CRDT.
			err = userMainCRDT.AddEffect(createUpd.AddMailbox.Value, createUpd.AddMailbox.Tag, true, true)
			if err != nil {

				// Perform clean up.
				stdlog.Printf("[imap.ApplyCRDTUpd] CREATE fail: %v", err)
				stdlog.Printf("[imap.Create] Removing added CRDT from mailbox structure and contents slice...\n")

				// Remove just added CRDT of new maildir from mailbox structure
				// and corresponding contents slice.
				delete(node.MailboxStructure[createUpd.User], createUpd.Mailbox)
				delete(node.MailboxContents[createUpd.User], createUpd.Mailbox)

				stdlog.Printf("[imap.Create] ... done. Removing just created Maildir completely...\n")

				// Attempt to remove Maildir.
				err = posMaildir.Remove()
				if err != nil {
					stdlog.Printf("[imap.ApplyCRDTUpd] ... failed to remove Maildir: %v", err)
					stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
				} else {
					stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
				}

				// Exit node.
				node.lock.Unlock()
				os.Exit(1)
			}

			// Unlock node.
			node.lock.Unlock()

		case "delete":

			// Parse received payload message into delete message struct.
			deleteUpd, err := comm.ParseDelete(opPayload)
			if err != nil {
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Error while parsing DELETE update from sync message: %v", err)
			}

			// Lock node exclusively.
			node.lock.Lock()

			// Save user's mailbox structure CRDT to more
			// conveniently use it hereafter.
			userMainCRDT := node.MailboxStructure[deleteUpd.User]["Structure"]

			// Construct remove set from received values.
			rmElements := make(map[string]string)
			for _, element := range deleteUpd.RmvMailbox {
				rmElements[element.Tag] = element.Value
			}

			// Remove received pairs from user's main CRDT.
			err = userMainCRDT.RemoveEffect(rmElements, true, true)
			if err != nil {
				node.lock.Unlock()
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to remove elements from user's main CRDT: %v", err)
			}

			// Remove CRDT from mailbox structure and corresponding
			// mail contents slice.
			delete(node.MailboxStructure[deleteUpd.User], deleteUpd.Mailbox)
			delete(node.MailboxContents[deleteUpd.User], deleteUpd.Mailbox)

			// Construct path to CRDT file to delete.
			delMailboxCRDTPath := filepath.Join(node.CRDTLayerRoot, deleteUpd.User, fmt.Sprintf("%s.log", deleteUpd.Mailbox))

			// Remove CRDT file of mailbox.
			err = os.Remove(delMailboxCRDTPath)
			if err != nil {
				node.lock.Unlock()
				stdlog.Fatalf("[imap.ApplyCRDTUpd] CRDT file of mailbox could not be deleted: %v", err)
			}

			// Remove files associated with deleted mailbox
			// from stable storage.
			delMaildir := maildir.Dir(filepath.Join(node.MaildirRoot, deleteUpd.User, deleteUpd.Mailbox))

			err = delMaildir.Remove()
			if err != nil {
				node.lock.Unlock()
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Maildir could not be deleted: %v", err)
			}

			// Unlock node.
			node.lock.Unlock()

		case "append":

			// Parse received payload message into append message struct.
			appendUpd, err := comm.ParseAppend(opPayload)
			if err != nil {
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Error while parsing APPEND update from sync message: %v", err)
			}

			// Lock node exclusively.
			node.lock.Lock()

			// Save user's mailbox structure CRDT to more
			// conveniently use it hereafter.
			userMainCRDT := node.MailboxStructure[appendUpd.User]["Structure"]

			// Check if specified mailbox from append message is present
			// in user's main CRDT on this node.
			if userMainCRDT.Lookup(appendUpd.Mailbox, true) {

				// Store concerned mailbox CRDT.
				userMailboxCRDT := node.MailboxStructure[appendUpd.User][appendUpd.Mailbox]

				// Check if mail is not yet present on this node.
				if userMailboxCRDT.Lookup(appendUpd.AddMail.Value, true) != true {

					// Construct path to new file.
					var appendFileName string
					if appendUpd.Mailbox == "INBOX" {
						appendFileName = filepath.Join(node.MaildirRoot, appendUpd.User, "cur", appendUpd.AddMail.Value)
					} else {
						appendFileName = filepath.Join(node.MaildirRoot, appendUpd.User, appendUpd.Mailbox, "cur", appendUpd.AddMail.Value)
					}

					// If so, place file contents at correct location.
					appendFile, err := os.Create(appendFileName)
					if err != nil {
						node.lock.Unlock()
						stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to create file for mail to append: %v", err)
					}

					_, err = appendFile.WriteString(appendUpd.AddMail.Contents)
					if err != nil {

						// Perform clean up.
						stdlog.Printf("[imap.ApplyCRDTUpd] APPEND fail: %v", err)
						stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created mail file...\n")

						// Remove just created mail file.
						err = os.Remove(appendFileName)
						if err != nil {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... failed: %v", err)
							stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
						} else {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
						}

						// Exit node.
						node.lock.Unlock()
						os.Exit(1)
					}

					// Sync contents to stable storage.
					err = appendFile.Sync()
					if err != nil {

						// Perform clean up.
						stdlog.Printf("[imap.ApplyCRDTUpd] APPEND fail: %v", err)
						stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created mail file...\n")

						// Remove just created mail file.
						err = os.Remove(appendFileName)
						if err != nil {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... failed: %v", err)
							stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
						} else {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
						}

						// Exit node.
						node.lock.Unlock()
						os.Exit(1)
					}

					// Append new mail to mailbox' contents CRDT.
					node.MailboxContents[appendUpd.User][appendUpd.Mailbox] = append(node.MailboxContents[appendUpd.User][appendUpd.Mailbox], appendUpd.AddMail.Value)

					// If succeeded, add new mail to mailbox' CRDT.
					err = userMailboxCRDT.AddEffect(appendUpd.AddMail.Value, appendUpd.AddMail.Tag, true, true)
					if err != nil {

						// Perform clean up.
						stdlog.Printf("[imap.ApplyCRDTUpd] APPEND fail: %v", err)
						stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created mail file...\n")

						// Remove just created mail file.
						err = os.Remove(appendFileName)
						if err != nil {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... failed: %v", err)
							stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
						} else {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
						}

						// Exit node.
						node.lock.Unlock()
						os.Exit(1)
					}
				} else {

					// Add new mail to mailbox' CRDT.
					err = userMailboxCRDT.AddEffect(appendUpd.AddMail.Value, appendUpd.AddMail.Tag, true, true)
					if err != nil {
						node.lock.Unlock()
						stdlog.Fatalf("[imap.ApplyCRDTUpd] APPEND fail: %s. Exiting.\n", err.Error())
					}
				}
			}

			// Unlock node.
			node.lock.Unlock()

		case "expunge":

			// Parse received payload message into expunge message struct.
			expungeUpd, err := comm.ParseExpunge(opPayload)
			if err != nil {
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Error while parsing EXPUNGE update from sync message: %v", err)
			}

			// Lock node exclusively.
			node.lock.Lock()

			// Save user's mailbox structure CRDT to more
			// conveniently use it hereafter.
			userMainCRDT := node.MailboxStructure[expungeUpd.User]["Structure"]

			// Check if specified mailbox from expunge message is
			// present in user's main CRDT on this node.
			if userMainCRDT.Lookup(expungeUpd.Mailbox, true) {

				// Store concerned mailbox CRDT.
				userMailboxCRDT := node.MailboxStructure[expungeUpd.User][expungeUpd.Mailbox]

				// Construct remove set from received values.
				rmElements := make(map[string]string)
				for _, element := range expungeUpd.RmvMail {
					rmElements[element.Tag] = element.Value
				}

				// Delete supplied elements from mailbox.
				err := userMailboxCRDT.RemoveEffect(rmElements, true, true)
				if err != nil {
					node.lock.Unlock()
					stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to remove mail elements from respective mailbox CRDT: %v", err)
				}

				// Check if just removed elements marked all
				// instances of mail file.
				if userMailboxCRDT.Lookup(expungeUpd.RmvMail[0].Value, true) != true {

					// Construct path to old file.
					var delFileName string
					if expungeUpd.Mailbox == "INBOX" {
						delFileName = filepath.Join(node.MaildirRoot, expungeUpd.User, "cur", expungeUpd.RmvMail[0].Value)
					} else {
						delFileName = filepath.Join(node.MaildirRoot, expungeUpd.User, expungeUpd.Mailbox, "cur", expungeUpd.RmvMail[0].Value)
					}

					// Remove the file.
					err := os.Remove(delFileName)
					if err != nil {
						node.lock.Unlock()
						stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to remove underlying mail file during EXPUNGE update: %v", err)
					}
				}

				for msgNum, msgName := range node.MailboxContents[expungeUpd.User][expungeUpd.Mailbox] {

					// Find removed mail file's sequence number.
					if msgName == expungeUpd.RmvMail[0].Value {

						// Delete mail's sequence number from contents structure.
						realMsgNum := msgNum + 1
						node.MailboxContents[expungeUpd.User][expungeUpd.Mailbox] = append(node.MailboxContents[expungeUpd.User][expungeUpd.Mailbox][:msgNum], node.MailboxContents[expungeUpd.User][expungeUpd.Mailbox][realMsgNum:]...)
					}
				}
			}

			// Unlock node.
			node.lock.Unlock()

		case "store":

			// Parse received payload message into store message struct.
			storeUpd, err := comm.ParseStore(opPayload)
			if err != nil {
				stdlog.Fatalf("[imap.ApplyCRDTUpd] Error while parsing STORE update from sync message: %v", err)
			}

			// Lock node exclusively.
			node.lock.Lock()

			// Save user's mailbox structure CRDT to more
			// conveniently use it hereafter.
			userMainCRDT := node.MailboxStructure[storeUpd.User]["Structure"]

			// Check if specified mailbox from store message is present
			// in user's main CRDT on this node.
			if userMainCRDT.Lookup(storeUpd.Mailbox, true) {

				// Store concerned mailbox CRDT.
				userMailboxCRDT := node.MailboxStructure[storeUpd.User][storeUpd.Mailbox]

				// Construct remove set from received values.
				rmElements := make(map[string]string)
				for _, element := range storeUpd.RmvMail {
					rmElements[element.Tag] = element.Value
				}

				// Delete supplied elements from mailbox.
				err := userMailboxCRDT.RemoveEffect(rmElements, true, true)
				if err != nil {
					node.lock.Unlock()
					stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to remove mail elements from respective mailbox CRDT: %v", err)
				}

				// Check if just removed elements marked all
				// instances of mail file.
				if userMailboxCRDT.Lookup(storeUpd.RmvMail[0].Value, true) != true {

					// Construct path to old file.
					var delFileName string
					if storeUpd.Mailbox == "INBOX" {
						delFileName = filepath.Join(node.MaildirRoot, storeUpd.User, "cur", storeUpd.RmvMail[0].Value)
					} else {
						delFileName = filepath.Join(node.MaildirRoot, storeUpd.User, storeUpd.Mailbox, "cur", storeUpd.RmvMail[0].Value)
					}

					// Remove the file.
					err := os.Remove(delFileName)
					if err != nil {
						node.lock.Unlock()
						stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to remove underlying mail file during STORE update: %v", err)
					}
				}

				// Check if new mail name is not yet present
				// on this node.
				if userMailboxCRDT.Lookup(storeUpd.AddMail.Value, true) != true {

					// Construct path to new file.
					var storeFileName string
					if storeUpd.Mailbox == "INBOX" {
						storeFileName = filepath.Join(node.MaildirRoot, storeUpd.User, "cur", storeUpd.AddMail.Value)
					} else {
						storeFileName = filepath.Join(node.MaildirRoot, storeUpd.User, storeUpd.Mailbox, "cur", storeUpd.AddMail.Value)
					}

					// If not yet present on node, place file
					// contents at correct location.
					storeFile, err := os.Create(storeFileName)
					if err != nil {
						node.lock.Unlock()
						stdlog.Fatalf("[imap.ApplyCRDTUpd] Failed to create file for mail of STORE operation: %v", err)
					}

					_, err = storeFile.WriteString(storeUpd.AddMail.Contents)
					if err != nil {

						// Perform clean up.
						stdlog.Printf("[imap.ApplyCRDTUpd] STORE fail: %v", err)
						stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created mail file...\n")

						// Remove just created mail file.
						err = os.Remove(storeFileName)
						if err != nil {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... failed: %v", err)
							stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
						} else {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
						}

						// Exit node.
						node.lock.Unlock()
						os.Exit(1)
					}

					// Sync contents to stable storage.
					err = storeFile.Sync()
					if err != nil {

						// Perform clean up.
						stdlog.Printf("[imap.ApplyCRDTUpd] STORE fail: %v", err)
						stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created mail file...\n")

						// Remove just created mail file.
						err = os.Remove(storeFileName)
						if err != nil {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... failed: %v", err)
							stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
						} else {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
						}

						// Exit node.
						node.lock.Unlock()
						os.Exit(1)
					}

					// If succeeded, add renamed mail to mailbox' CRDT.
					err = userMailboxCRDT.AddEffect(storeUpd.AddMail.Value, storeUpd.AddMail.Tag, true, true)
					if err != nil {

						// Perform clean up.
						stdlog.Printf("[imap.ApplyCRDTUpd] STORE fail: %v", err)
						stdlog.Printf("[imap.ApplyCRDTUpd] Removing just created mail file...\n")

						// Remove just created mail file.
						err = os.Remove(storeFileName)
						if err != nil {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... failed: %v", err)
							stdlog.Printf("[imap.ApplyCRDTUpd] Exiting.\n")
						} else {
							stdlog.Printf("[imap.ApplyCRDTUpd] ... done. Exiting.\n")
						}

						// Exit node.
						node.lock.Unlock()
						os.Exit(1)
					}
				} else {

					// Add renamed mail to mailbox' CRDT.
					err = userMailboxCRDT.AddEffect(storeUpd.AddMail.Value, storeUpd.AddMail.Tag, true, true)
					if err != nil {
						node.lock.Unlock()
						stdlog.Fatalf("[imap.ApplyCRDTUpd] STORE fail: %s. Exiting.\n", err.Error())
					}
				}

				for msgNum, msgName := range node.MailboxContents[storeUpd.User][storeUpd.Mailbox] {

					// Find old mail file's sequence number.
					if msgName == storeUpd.RmvMail[0].Value {

						// Replace old file name with renamed new one.
						node.MailboxContents[storeUpd.User][storeUpd.Mailbox][msgNum] = storeUpd.AddMail.Value
					}
				}
			}

			// Unlock node.
			node.lock.Unlock()

		}

		// Signal receiver that update was performed.
		doneChan <- struct{}{}
	}
}

// Mailbox functions

// Select sets the current mailbox based on supplied
// payload to user-instructed value. A return value of
// this function does not indicate whether the command
// was successfully handled according to IMAP semantics,
// but rather whether a fatal error occurred or a complete
// answer could been sent. So, in case of an user error
// (e.g. a missing mailbox to select) but otherwise correct
// handling, this function would send a useful message to
// the client and still return true.
func (node *IMAPNode) Select(c *IMAPConnection, req *Request, syncChan chan string) bool {

	if (c.State != Authenticated) && (c.State != Mailbox) {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command SELECT cannot be executed in this state", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	if len(req.Payload) < 1 {

		// If no mailbox to select was specified in payload,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command SELECT was sent without a mailbox to select", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every whitespace character.
	mailboxes := strings.Split(req.Payload, " ")

	if len(mailboxes) != 1 {

		// If there were more than two names supplied to select,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command SELECT was sent with multiple mailbox names instead of only one", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Save maildir for later use.
	mailboxPath := c.UserMaildirPath

	// If any other mailbox than INBOX was specified,
	// append it to mailbox in order to check it.
	if mailboxes[0] != "INBOX" {
		mailboxPath = filepath.Join(mailboxPath, mailboxes[0])
	}

	// Transform into real Maildir.
	mailbox := maildir.Dir(mailboxPath)

	// Check if mailbox is existing and a conformant maildir folder.
	err := mailbox.Check()
	if err != nil {

		// If specified maildir did not turn out to be a valid one,
		// this is a client error. Return NO statement.
		err := c.InternalSend(true, fmt.Sprintf("%s NO SELECT failure, not a valid Maildir folder", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Set selected mailbox in connection to supplied one
	// and advance IMAP state of connection to Mailbox.
	c.State = Mailbox
	c.SelectedMailbox = mailboxes[0]

	node.lock.RLock()
	defer node.lock.RUnlock()

	// Store contents structure of selected mailbox
	// for later convenient use.
	selMailboxContents := node.MailboxContents[c.UserName][c.SelectedMailbox]

	// Count how many mails do not have the \Seen
	// flag attached, i.e. recent mails.
	recentMails := 0
	for _, mail := range selMailboxContents {

		// Retrieve flags of mail.
		mailFlags, err := mailbox.Flags(mail, false)
		if err != nil {
			c.Error("Error while retrieving flags for mail", err)
			return false
		}

		// Check if \Seen flag is present.
		if strings.ContainsRune(mailFlags, 'S') != true {
			recentMails++
		}
	}

	// TODO: Add other SELECT response elements if needed.

	// Send answer to requesting client.
	err = c.InternalSend(true, fmt.Sprintf("* %d EXISTS\r\n* %d RECENT\r\n* FLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)\r\n* OK [PERMANENTFLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)]\r\n%s OK [READ-WRITE] SELECT completed", len(selMailboxContents), recentMails, req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Create attempts to create a mailbox with name
// taken from payload of request.
func (node *IMAPNode) Create(c *IMAPConnection, req *Request, syncChan chan string) bool {

	if (c.State != Authenticated) && (c.State != Mailbox) {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command APPEND cannot be executed in this state", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every space character.
	posMailboxes := strings.Split(req.Payload, " ")

	if len(posMailboxes) != 1 {

		// If payload did not contain exactly one element,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command CREATE was not sent with exactly one parameter", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Trim supplied mailbox name of hierarchy separator if
	// it was sent with a trailing one.
	posMailbox := strings.TrimSuffix(posMailboxes[0], node.Config.IMAP.HierarchySeparator)

	if strings.ToUpper(posMailbox) == "INBOX" {

		// If mailbox to-be-created was named INBOX,
		// this is a client error. Return NO response.
		err := c.InternalSend(true, fmt.Sprintf("%s NO New mailbox cannot be named INBOX", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	node.lock.RLock()

	// Save user's mailbox structure CRDT to more
	// conveniently use it hereafter.
	userMainCRDT := node.MailboxStructure[c.UserName]["Structure"]

	if userMainCRDT.Lookup(posMailbox, true) {

		// If mailbox to-be-created already exists for user,
		// this is a client error. Return NO response.
		err := c.InternalSend(true, fmt.Sprintf("%s NO New mailbox cannot be named after already existing mailbox", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			node.lock.RUnlock()
			return false
		}

		node.lock.RUnlock()

		return true
	}

	// Create a new Maildir on stable storage.
	posMaildir := maildir.Dir(filepath.Join(c.UserMaildirPath, posMailbox))

	err := posMaildir.Create()
	if err != nil {
		c.Error("Error while creating Maildir for new mailbox", err)
		node.lock.RUnlock()
		return false
	}

	// Construct path to new CRDT file.
	posMailboxCRDTPath := filepath.Join(c.UserCRDTPath, fmt.Sprintf("%s.log", posMailbox))

	// Initialize new ORSet for new mailbox.
	posMailboxCRDT, err := crdt.InitORSetWithFile(posMailboxCRDTPath)
	if err != nil {

		// Perform clean up.
		stdlog.Printf("[imap.Create] Fail: %v", err)
		stdlog.Printf("[imap.Create] Removing just created Maildir completely...\n")

		// Attempt to remove Maildir.
		err = posMaildir.Remove()
		if err != nil {
			stdlog.Printf("[imap.Create] ... failed to remove Maildir: %v", err)
			stdlog.Printf("[imap.Create] Exiting.\n")
		} else {
			stdlog.Printf("[imap.Create] ... done. Exiting.\n")
		}

		node.lock.RUnlock()

		// Exit node.
		os.Exit(1)
	}

	node.lock.RUnlock()
	node.lock.Lock()

	// Place newly created CRDT in mailbox structure.
	node.MailboxStructure[c.UserName][posMailbox] = posMailboxCRDT

	// Initialize contents slice for new mailbox to track
	// message sequence numbers in it.
	node.MailboxContents[c.UserName][posMailbox] = make([]string, 0, 6)

	// If succeeded, add a new folder in user's main CRDT
	// and synchronise it to other replicas.
	err = userMainCRDT.Add(posMailbox, func(payload string) {
		syncChan <- fmt.Sprintf("create|%s|%s|%s", c.UserName, base64.StdEncoding.EncodeToString([]byte(posMailbox)), payload)
	})
	if err != nil {

		// Perform clean up.
		stdlog.Printf("[imap.Create] Fail: %v", err)
		stdlog.Printf("[imap.Create] Removing added CRDT from mailbox structure...\n")

		// Remove just added CRDT of new maildir from mailbox structure
		// and corresponding contents slice.
		delete(node.MailboxStructure[c.UserName], posMailbox)
		delete(node.MailboxContents[c.UserName], posMailbox)

		stdlog.Printf("[imap.Create] ... done. Removing just created Maildir completely...\n")

		// Attempt to remove Maildir.
		err = posMaildir.Remove()
		if err != nil {
			stdlog.Printf("[imap.Create] ... failed to remove Maildir: %v", err)
			stdlog.Printf("[imap.Create] Exiting.\n")
		} else {
			stdlog.Printf("[imap.Create] ... done. Exiting.\n")
		}

		node.lock.Unlock()

		// Exit node.
		os.Exit(1)
	}

	node.lock.Unlock()

	// Send success answer.
	err = c.InternalSend(true, fmt.Sprintf("%s OK CREATE completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Delete attempts to remove an existing mailbox with
// all included content in CRDT as well as file system.
func (node *IMAPNode) Delete(c *IMAPConnection, req *Request, syncChan chan string) bool {

	if (c.State != Authenticated) && (c.State != Mailbox) {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command DELETE cannot be executed in this state", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every space character.
	delMailboxes := strings.Split(req.Payload, " ")

	if len(delMailboxes) != 1 {

		// If payload did not contain exactly one element,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command DELETE was not sent with exactly one parameter", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Trim supplied mailbox name of hierarchy separator if
	// it was sent with a trailing one.
	delMailbox := strings.TrimSuffix(delMailboxes[0], node.Config.IMAP.HierarchySeparator)

	if strings.ToUpper(delMailbox) == "INBOX" {

		// If mailbox to-be-deleted was named INBOX,
		// this is a client error. Return NO response.
		err := c.InternalSend(true, fmt.Sprintf("%s NO Forbidden to delete INBOX", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	node.lock.Lock()
	defer node.lock.Unlock()

	// Save user's mailbox structure CRDT to more
	// conveniently use it hereafter.
	userMainCRDT := node.MailboxStructure[c.UserName]["Structure"]

	// TODO: Add routines to take care of mailboxes that
	//       are tagged with a \Noselect tag.

	// Remove element from user's main CRDT and send out
	// remove update operations to all other replicas.
	err := userMainCRDT.Remove(delMailbox, func(payload string) {
		syncChan <- fmt.Sprintf("delete|%s|%s|%s", c.UserName, base64.StdEncoding.EncodeToString([]byte(delMailbox)), payload)
	})
	if err != nil {

		// Check if error was caused by client, trying to
		// delete an non-existent mailbox.
		if err.Error() == "element to be removed not found in set" {

			// If so, return a NO response.
			err := c.InternalSend(true, fmt.Sprintf("%s NO Cannot delete folder that does not exist", req.Tag))
			if err != nil {
				c.Error("Encountered send error", err)
				return false
			}

			return true
		}

		// Otherwise, this is a write-back error of the updated CRDT
		// log file. Reverting actions were already taken, log error.
		stdlog.Printf("[imap.Delete] Failed to remove elements from user's main CRDT: %v", err)

		// Exit node.
		os.Exit(1)
	}

	// Remove CRDT from mailbox structure and corresponding
	// mail contents slice.
	delete(node.MailboxStructure[c.UserName], delMailbox)
	delete(node.MailboxContents[c.UserName], delMailbox)

	// Construct path to CRDT file to delete.
	delMailboxCRDTPath := filepath.Join(c.UserCRDTPath, fmt.Sprintf("%s.log", delMailbox))

	// Remove CRDT file of mailbox.
	err = os.Remove(delMailboxCRDTPath)
	if err != nil {
		// TODO: Maybe think about better way to clean up here?
		c.Error("Error while deleting CRDT file of mailbox", err)
		return false
	}

	// Remove files associated with deleted mailbox
	// from stable storage.
	delMaildir := maildir.Dir(filepath.Join(c.UserMaildirPath, delMailbox))

	err = delMaildir.Remove()
	if err != nil {
		// TODO: Maybe think about better way to clean up here?
		c.Error("Error while deleting Maildir", err)
		return false
	}

	// Send success answer.
	err = c.InternalSend(true, fmt.Sprintf("%s OK DELETE completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// List allows clients to learn about the mailboxes
// available and also returns the hierarchy delimiter.
func (node *IMAPNode) List(c *IMAPConnection, req *Request, syncChan chan string) bool {

	if (c.State != Authenticated) && (c.State != Mailbox) {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command LIST cannot be executed in this state", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every space character.
	listArgs := strings.Split(req.Payload, " ")

	if len(listArgs) != 2 {

		// If payload did not contain between exactly two elements,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command LIST was not sent with exactly two arguments", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	if (listArgs[1] != "%") && (listArgs[1] != "*") {

		// If second argument is not one of two wildcards,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command LIST needs either '%%' or '*' as mailbox name", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	node.lock.RLock()

	// Save user's mailbox structure CRDT to more
	// conveniently use it hereafter.
	userMainCRDT := node.MailboxStructure[c.UserName]

	// Reserve space for answer.
	listAnswerLines := make([]string, 0, (len(userMainCRDT) - 1))

	for mailbox := range userMainCRDT {

		// Do not consider structure element.
		if mailbox != "Structure" {

			// Split currently considered mailbox name at
			// defined hierarchy separator.
			mailboxParts := strings.Split(mailbox, node.Config.IMAP.HierarchySeparator)

			if (listArgs[1] == "*") || (len(mailboxParts) == 1) {

				// Either always include a mailbox in the response
				// or only when it is a top level mailbox.
				listAnswerLines = append(listAnswerLines, fmt.Sprintf("* LIST () \"%s\" %s", node.Config.IMAP.HierarchySeparator, mailbox))
			}
		}
	}

	node.lock.RUnlock()

	// Send out LIST response lines.
	for _, listAnswerLine := range listAnswerLines {

		err := c.InternalSend(true, listAnswerLine)
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}
	}

	// Send success answer.
	err := c.InternalSend(true, fmt.Sprintf("%s OK LIST completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Mail functions

// Append puts supplied message into specified mailbox.
func (node *IMAPNode) Append(c *IMAPConnection, req *Request, syncChan chan string) bool {

	var mailbox string
	var flagsRaw string
	var dateTimeRaw string
	var numBytesRaw string

	if (c.State != Authenticated) && (c.State != Mailbox) {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command APPEND cannot be executed in this state", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every space character.
	appendArgs := strings.Split(req.Payload, " ")
	lenAppendArgs := len(appendArgs)

	if (lenAppendArgs < 2) || (lenAppendArgs > 4) {

		// If payload did not contain between two and four
		// elements, this is a client error.
		// Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command APPEND was not sent with appropriate number of parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Depending on amount of arguments, store them.
	switch lenAppendArgs {

	case 2:
		mailbox = appendArgs[0]
		numBytesRaw = appendArgs[1]

	case 3:
		mailbox = appendArgs[0]
		flagsRaw = appendArgs[1]
		numBytesRaw = appendArgs[2]

	case 4:
		mailbox = appendArgs[0]
		flagsRaw = appendArgs[1]
		dateTimeRaw = appendArgs[2]
		numBytesRaw = appendArgs[3]
	}

	// If user specified inbox, set it accordingly.
	if strings.ToUpper(mailbox) == "INBOX" {
		mailbox = "INBOX"
	}

	// If flags were supplied, parse them.
	if flagsRaw != "" {

		_, err := ParseFlags(flagsRaw)
		if err != nil {

			// Parsing flags from APPEND request produced
			// an error. Return tagged BAD response.
			err := c.InternalSend(true, fmt.Sprintf("%s BAD %s", req.Tag, err.Error()))
			if err != nil {
				c.Error("Encountered send error", err)
				return false
			}

			return true
		}

		// TODO: Do something with these flags.
	}

	// If date-time was supplied, parse it.
	if dateTimeRaw != "" {
		// TODO: Parse time and do something with it.
	}

	// Parse out how many bytes we are expecting.
	numBytesString := strings.TrimPrefix(numBytesRaw, "{")
	numBytesString = strings.TrimSuffix(numBytesString, "}")

	// Convert string number to int.
	numBytes, err := strconv.Atoi(numBytesString)
	if err != nil {

		// If we were not able to parse out the number,
		// it was probably a client error. Send tagged BAD.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command APPEND did not contain proper literal data byte number", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	node.lock.RLock()

	// Save user's mailbox structure CRDT to more
	// conveniently use it hereafter.
	userMainCRDT := node.MailboxStructure[c.UserName]["Structure"]

	if userMainCRDT.Lookup(mailbox, true) != true {

		// If mailbox to append message to does not exist,
		// this is a client error. Return NO response.
		err := c.InternalSend(true, fmt.Sprintf("%s NO [TRYCREATE] Mailbox to append to does not exist", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			node.lock.RUnlock()
			return false
		}

		node.lock.RUnlock()

		return true
	}

	// Send command continuation to client.
	err = c.InternalSend(true, "+ Ready for literal data")
	if err != nil {
		c.Error("Encountered send error", err)
		node.lock.RUnlock()
		return false
	}

	// Signal proxying distributor that we expect an
	// inbound answer from the client.
	err = c.SignalAwaitingLiteral(true, numBytes)
	if err != nil {
		c.Error("Encountered send error", err)
		node.lock.RUnlock()
		return false
	}

	// Reserve space for exact amount of expected data.
	msgBuffer := make([]byte, numBytes)

	// Read in that amount from connection to distributor.
	_, err = io.ReadFull(c.IncReader, msgBuffer)
	if err != nil {
		c.Error("Encountered error while reading distributor literal data", err)
		node.lock.RUnlock()
		return false
	}

	// Construct path to maildir on node.
	var appMaildir maildir.Dir
	if mailbox == "INBOX" {
		appMaildir = maildir.Dir(c.UserMaildirPath)
	} else {
		appMaildir = maildir.Dir(filepath.Join(c.UserMaildirPath, mailbox))
	}

	node.lock.RUnlock()
	node.lock.Lock()
	defer node.lock.Unlock()

	// Open a new Maildir delivery.
	appDelivery, err := appMaildir.NewDelivery()
	if err != nil {
		c.Error("Error during delivery creation", err)
		return false
	}

	// Write actual message contents to file.
	err = appDelivery.Write(msgBuffer)
	if err != nil {
		c.Error("Error while writing message during delivery", err)
		return false
	}

	// Close and move just created message.
	newKey, err := appDelivery.Close()
	if err != nil {
		c.Error("Error while finishing delivery of new message", err)
		return false
	}

	// Follow Maildir's renaming procedure.
	_, err = appMaildir.Unseen()
	if err != nil {
		c.Error("Could not Unseen() recently delivered messages", err)
		return false
	}

	// Find file name of just delivered mail.
	mailFileNamePath, err := appMaildir.Filename(newKey)
	if err != nil {
		c.Error("Finding file name of new message failed", err)
		return false
	}
	mailFileName := filepath.Base(mailFileNamePath)

	// Retrieve CRDT of mailbox to append mail to.
	appMailboxCRDT := node.MailboxStructure[c.UserName][mailbox]

	// Append new mail to mailbox' contents CRDT.
	node.MailboxContents[c.UserName][mailbox] = append(node.MailboxContents[c.UserName][mailbox], mailFileName)

	// Add new mail to mailbox' CRDT and send update
	// message to other replicas.
	err = appMailboxCRDT.Add(mailFileName, func(payload string) {
		syncChan <- fmt.Sprintf("append|%s|%s|%s;%s", c.UserName, base64.StdEncoding.EncodeToString([]byte(mailbox)), payload, base64.StdEncoding.EncodeToString(msgBuffer))
	})
	if err != nil {

		// Perform clean up.
		stdlog.Printf("[imap.Append] Fail: %v", err)
		stdlog.Printf("[imap.Append] Removing just appended mail message...\n")

		err := os.Remove(mailFileNamePath)
		if err != nil {
			stdlog.Printf("[imap.Append] ... failed: %v", err)
			stdlog.Printf("[imap.Append] Exiting.\n")
		} else {
			stdlog.Printf("[imap.Append] ... done. Exiting.\n")
		}

		// Exit node.
		os.Exit(1)
	}

	// Send success answer.
	err = c.InternalSend(true, fmt.Sprintf("%s OK APPEND completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Expunge deletes messages permanently from currently
// selected mailbox that have been flagged as Deleted
// prior to calling this function.
func (node *IMAPNode) Expunge(c *IMAPConnection, req *Request, syncChan chan string) bool {

	if c.State != Mailbox {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD No mailbox selected to expunge", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	if len(req.Payload) > 0 {

		// If payload was not empty to EXPUNGE command,
		// this is a client error. Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command EXPUNGE was sent with extra parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Construct path to Maildir to expunge.
	var expMaildir maildir.Dir
	if c.SelectedMailbox == "INBOX" {
		expMaildir = maildir.Dir(filepath.Join(c.UserMaildirPath, "cur"))
	} else {
		expMaildir = maildir.Dir(filepath.Join(c.UserMaildirPath, c.SelectedMailbox, "cur"))
	}

	// Reserve space for mails to expunge.
	expMailNums := make([]int, 0, 6)

	// Declare variable to contain answers of
	// individual remove operations.
	var expAnswerLines []string

	node.lock.RLock()

	// Save all mails possibly to delete and
	// amount of these files.
	expMails := node.MailboxContents[c.UserName][c.SelectedMailbox]
	numExpMails := len(expMails)

	node.lock.RUnlock()

	// Only do the work if there are any mails
	// present in mailbox.
	if numExpMails > 0 {

		node.lock.Lock()

		// Iterate over all mail files in reverse order.
		for i := (numExpMails - 1); i >= 0; i-- {

			// Retrieve all flags of fetched mail.
			mailFlags, err := expMaildir.Flags(expMails[i], false)
			if err != nil {
				c.Error("Encountered error while retrieving flags for expunging mails", err)
				node.lock.Unlock()
				return false
			}

			// Check for presence of \Deleted flag and
			// add to delete set if found.
			if strings.ContainsRune(mailFlags, 'T') {
				expMailNums = append(expMailNums, i)
			}
		}

		// Reserve space for answer lines.
		expAnswerLines = make([]string, 0, len(expMailNums))

		// Retrieve CRDT of mailbox to expunge.
		expMailboxCRDT := node.MailboxStructure[c.UserName][c.SelectedMailbox]

		for _, msgNum := range expMailNums {

			// Remove each mail to expunge from mailbox CRDT.
			err := expMailboxCRDT.Remove(expMails[msgNum], func(payload string) {
				syncChan <- fmt.Sprintf("expunge|%s|%s|%s", c.UserName, base64.StdEncoding.EncodeToString([]byte(c.SelectedMailbox)), payload)
			})
			if err != nil {

				// This is a write-back error of the updated mailbox CRDT
				// log file. Reverting actions were already taken, log error.
				stdlog.Printf("[imap.Expunge] Failed to remove mails from user's selected mailbox CRDT: %v", err)

				node.lock.Unlock()

				// Exit node.
				os.Exit(1)
			}

			// Construct path to file.
			expMailPath := filepath.Join(string(expMaildir), expMails[msgNum])

			// Remove the file.
			err = os.Remove(expMailPath)
			if err != nil {
				c.Error("Error while removing expunged mail file from stable storage", err)
				node.lock.Unlock()
				return false
			}

			// Immediately remove mail from contents structure.
			realMsgNum := msgNum + 1
			expMails = append(expMails[:msgNum], expMails[realMsgNum:]...)

			// Append individual remove answer to answer lines.
			expAnswerLines = append(expAnswerLines, fmt.Sprintf("* %d EXPUNGE", realMsgNum))
		}

		node.lock.Unlock()

		// Send out FETCH part with new flags.
		for _, expAnswerLine := range expAnswerLines {

			err := c.InternalSend(true, expAnswerLine)
			if err != nil {
				c.Error("Encountered send error", err)
				return false
			}
		}
	}

	// Send success answer.
	err := c.InternalSend(true, fmt.Sprintf("%s OK EXPUNGE completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Store takes in message sequence numbers and some set
// of flags to change in those messages and changes the
// attributes for these mails throughout the system.
func (node *IMAPNode) Store(c *IMAPConnection, req *Request, syncChan chan string) bool {

	// Set updated flags list indicator
	// initially to false.
	silent := false

	if c.State != Mailbox {

		// If connection was not in correct state when this
		// command was executed, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD No mailbox selected for store", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every space character.
	storeArgs := strings.SplitN(req.Payload, " ", 3)

	if len(storeArgs) < 3 {

		// If payload did not contain at least three
		// elements, this is a client error.
		// Return BAD statement.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Command STORE was not sent with three parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	node.lock.RLock()

	// Retrieve number of messages in mailbox.
	lenMailboxContents := len(node.MailboxContents[c.UserName][c.SelectedMailbox])

	node.lock.RUnlock()

	// Parse sequence numbers argument.
	msgNums, err := ParseSeqNumbers(storeArgs[0], lenMailboxContents)
	if err != nil {

		// Parsing sequence numbers from STORE request produced
		// an error. Return tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD %s", req.Tag, err.Error()))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Parse data item type.
	dataItemType := storeArgs[1]

	if (dataItemType != "FLAGS") && (dataItemType != "FLAGS.SILENT") &&
		(dataItemType != "+FLAGS") && (dataItemType != "+FLAGS.SILENT") &&
		(dataItemType != "-FLAGS") && (dataItemType != "-FLAGS.SILENT") {

		// If supplied data item type is none of the
		// supported ones, this is a client error.
		// Send tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD Unknown data item type specified", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// If client requested not to receive the updated
	// flags list, set indicator to false.
	if strings.HasSuffix(dataItemType, ".SILENT") {
		silent = true
	}

	// Parse flag argument.
	flags, err := ParseFlags(storeArgs[2])
	if err != nil {

		// Parsing flags from STORE request produced
		// an error. Return tagged BAD response.
		err := c.InternalSend(true, fmt.Sprintf("%s BAD %s", req.Tag, err.Error()))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Set currently selected mailbox with respect to special
	// case of INBOX as current location.
	var selectedMailbox string
	if c.SelectedMailbox == "INBOX" {
		selectedMailbox = c.UserMaildirPath
	} else {
		selectedMailbox = filepath.Join(c.UserMaildirPath, c.SelectedMailbox)
	}

	// Prepare answer slice.
	answerLines := make([]string, 0, len(msgNums))

	// Construct path and Maildir for selected mailbox.
	mailMaildir := maildir.Dir(selectedMailbox)

	for _, msgNum := range msgNums {

		var rmvElements string

		// Initialize runes slice for new flags of mail.
		newMailFlags := make([]rune, 0, 5)

		// Depending on the presence of various standard
		// flags extend newMailFlags further.

		if _, found := flags["\\Draft"]; found {
			newMailFlags = append(newMailFlags, 'D')
		}

		if _, found := flags["\\Flagged"]; found {
			newMailFlags = append(newMailFlags, 'F')
		}

		if _, found := flags["\\Answered"]; found {
			newMailFlags = append(newMailFlags, 'R')
		}

		if _, found := flags["\\Seen"]; found {
			newMailFlags = append(newMailFlags, 'S')
		}

		if _, found := flags["\\Deleted"]; found {
			newMailFlags = append(newMailFlags, 'T')
		}

		node.lock.RLock()

		// Retrieve mail file name.
		mailFileName := node.MailboxContents[c.UserName][c.SelectedMailbox][msgNum]

		// Read message contents from file.
		mailFileContents, err := ioutil.ReadFile(filepath.Join(selectedMailbox, "cur", mailFileName))
		if err != nil {
			c.Error("Error while reading in mail file contents in STORE operation", err)
			return false
		}

		// Retrieve flags included in mail file name.
		mailFlags, err := mailMaildir.Flags(mailFileName, false)
		if err != nil {
			c.Error("Error while retrieving flags from mail file", err)
			return false
		}

		node.lock.RUnlock()

		// Check if supplied flags should be added
		// to existing flags if not yet present.
		if (dataItemType == "+FLAGS") || (dataItemType == "+FLAGS.SILENT") {

			newMailFlagsString := string(newMailFlags)

			for _, char := range mailFlags {

				// Iterate over all characters of the currently present
				// flags of the mail and check if they are present in
				// new flags string as well. If not, add it.
				if strings.ContainsRune(newMailFlagsString, char) != true {
					newMailFlags = append(newMailFlags, char)
				}
			}
		}

		// Check if supplied flags should be removed
		// from existing flags if they are present.
		if (dataItemType == "-FLAGS") || (dataItemType == "-FLAGS.SILENT") {

			tmpNewMailFlags := mailFlags

			for _, char := range newMailFlags {

				// Iterate over all characters of the supplied new
				// flags and check if they are present in current
				// flags string. If so, remove them from intermediate
				// variable that holds former flags.
				if strings.ContainsRune(mailFlags, char) {
					tmpNewMailFlags = strings.Replace(tmpNewMailFlags, string(char), "", -1)
				}
			}

			// Reset new flags slice to reduced intermediate value.
			newMailFlags = []rune(tmpNewMailFlags)
		}

		// Check if we really have to perform and update
		// across the system or if we can save the energy.
		if mailFlags != string(newMailFlags) {

			node.lock.Lock()

			// Set just constructed new flags string in mail's
			// file name (renaming it).
			newMailFileName, err := mailMaildir.SetFlags(mailFileName, string(newMailFlags), false)
			if err != nil {
				c.Error("Error renaming mail file in STORE operation", err)
				node.lock.Unlock()
				return false
			}

			// Save CRDT of mailbox.
			storeMailboxCRDT := node.MailboxStructure[c.UserName][c.SelectedMailbox]

			// First, remove the former name of the mail file
			// but do not yet send out an update operation.
			err = storeMailboxCRDT.Remove(mailFileName, func(payload string) {
				rmvElements = payload
			})
			if err != nil {

				// This is a write-back error of the updated mailbox CRDT
				// log file. Reverting actions were already taken, log error.
				stdlog.Printf("[imap.Store] Failed to remove old mail name from selected mailbox CRDT: %v", err)

				node.lock.Unlock()

				// Exit node.
				os.Exit(1)
			}

			// Second, add the new mail file's name and finally
			// instruct all other nodes to do the same.
			err = storeMailboxCRDT.Add(newMailFileName, func(payload string) {
				syncChan <- fmt.Sprintf("store|%s|%s|%s|%s;%s", c.UserName, base64.StdEncoding.EncodeToString([]byte(c.SelectedMailbox)), rmvElements, payload, base64.StdEncoding.EncodeToString(mailFileContents))
			})
			if err != nil {

				// This is a write-back error of the updated mailbox CRDT
				// log file. Reverting actions were already taken, log error.
				stdlog.Printf("[imap.Store] Failed to add renamed mail name to selected mailbox CRDT: %v", err)

				node.lock.Unlock()

				// Exit node.
				os.Exit(1)
			}

			// If we are done with that, also replace the mail's
			// file name in the corresponding contents slice.
			node.MailboxContents[c.UserName][c.SelectedMailbox][msgNum] = newMailFileName

			node.lock.Unlock()
		}

		// Check if client requested update information.
		if silent != true {

			// Build new flags list for answer.
			answerFlagString := ""

			for _, newFlag := range newMailFlags {

				switch newFlag {

				case 'D':

					if answerFlagString == "" {
						answerFlagString = "\\Draft"
					} else {
						answerFlagString = fmt.Sprintf("%s \\Draft", answerFlagString)
					}

				case 'F':

					if answerFlagString == "" {
						answerFlagString = "\\Flagged"
					} else {
						answerFlagString = fmt.Sprintf("%s \\Flagged", answerFlagString)
					}

				case 'R':

					if answerFlagString == "" {
						answerFlagString = "\\Answered"
					} else {
						answerFlagString = fmt.Sprintf("%s \\Answered", answerFlagString)
					}

				case 'S':

					if answerFlagString == "" {
						answerFlagString = "\\Seen"
					} else {
						answerFlagString = fmt.Sprintf("%s \\Seen", answerFlagString)
					}

				case 'T':

					if answerFlagString == "" {
						answerFlagString = "\\Deleted"
					} else {
						answerFlagString = fmt.Sprintf("%s \\Deleted", answerFlagString)
					}

				}

			}

			// Append this file's FETCH answer.
			realMsgNum := msgNum + 1
			answerLines = append(answerLines, fmt.Sprintf("* %d FETCH (FLAGS (%s))", realMsgNum, answerFlagString))
		}
	}

	// Check if client requested update information.
	if silent != true {

		// Send out FETCH part with new flags.
		for _, answerLine := range answerLines {

			err = c.InternalSend(true, answerLine)
			if err != nil {
				c.Error("Encountered send error", err)
				return false
			}
		}
	}

	// Send success answer.
	err = c.InternalSend(true, fmt.Sprintf("%s OK STORE completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}
