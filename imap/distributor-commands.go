package imap

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Functions

// Capability handles the IMAP CAPABILITY command.
// It outputs the supported actions in the current state.
func (distr *Distributor) Capability(c *Connection, req *Request) bool {

	if len(req.Payload) > 0 {

		// If payload was not empty to CAPABILITY command,
		// this is a client error. Return BAD statement.
		err := c.Send(true, fmt.Sprintf("%s BAD Command CAPABILITY was sent with extra parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Send mandatory capability options.
	// This means, AUTH=PLAIN is allowed and nothing else.
	// STARTTLS will be answered but is not listed as
	// each connection already is a TLS connection.
	err := c.Send(true, fmt.Sprintf("* CAPABILITY IMAP4rev1 AUTH=PLAIN\r\n%s OK CAPABILITY completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	// TODO: Change returned capabilities based on IMAP state of
	//       connection, e.g. more capabilities if authenticated.

	return true
}

// Logout correctly ends a connection with a client.
// Also necessarily created management structures will
// get shut down gracefully.
func (distr *Distributor) Logout(c *Connection, req *Request) bool {

	if len(req.Payload) > 0 {

		// If payload was not empty to LOGOUT command,
		// this is a client error. Return BAD statement.
		err := c.Send(true, fmt.Sprintf("%s BAD Command LOGOUT was sent with extra parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// If already a worker was assigned, signal logout.
	if c.OutConn != nil {

		// Signal to worker node that session is done.
		err := c.SignalSessionDone(false)
		if err != nil {
			c.Error("Encountered send error when distributor was signalling end to worker", err)
			return false
		}
	}

	// Signal success to client.
	err := c.Send(true, fmt.Sprintf("* BYE Terminating connection\r\n%s OK LOGOUT completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// StartTLS states on IMAP STARTTLS command
// that current connection is already encrypted.
func (distr *Distributor) StartTLS(c *Connection, req *Request) bool {

	if len(req.Payload) > 0 {

		// If payload was not empty to STARTTLS command,
		// this is a client error. Return BAD statement.
		err := c.Send(true, fmt.Sprintf("%s BAD Command STARTTLS was sent with extra parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// As the connection is already TLS encrypted,
	// tell client that a TLS session is active.
	err := c.Send(true, fmt.Sprintf("%s BAD TLS is already active", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Login performs the authentication mechanism specified
// as part of the distributor config.
func (distr *Distributor) Login(c *Connection, req *Request) bool {

	if c.OutConn != nil {

		// Connection was already once authenticated,
		// cannot do that a second time, client error.
		// Send tagged BAD response.
		err := c.Send(true, fmt.Sprintf("%s BAD Command LOGIN cannot be executed in this state", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Split payload on every space character.
	userCredentials := strings.Split(req.Payload, " ")

	if len(userCredentials) != 2 {

		// If payload did not contain exactly two elements,
		// this is a client error. Return BAD statement.
		err := c.Send(true, fmt.Sprintf("%s BAD Command LOGIN was not sent with exactly two parameters", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	id, clientID, err := distr.AuthAdapter.AuthenticatePlain(userCredentials[0], userCredentials[1], c.IncConn.RemoteAddr().String())
	if err != nil {

		// If supplied credentials failed to authenticate client,
		// they are invalid. Return NO statement.
		err := c.Send(true, fmt.Sprintf("%s NO Name and / or password wrong", req.Tag))
		if err != nil {
			c.Error("Encountered send error", err)
			return false
		}

		return true
	}

	// Find worker node responsible for this connection.
	respWorker, err := distr.AuthAdapter.GetWorkerForUser(distr.Config.Workers, id)
	if err != nil {
		c.Error("Authentication error", err)
		return false
	}

	distr.lock.RLock()

	// Store worker connection information.
	workerIP := distr.Config.Workers[respWorker].PublicIP
	workerPort := distr.Config.Workers[respWorker].MailPort

	distr.lock.RUnlock()

	// Prepare address string of storage node for
	// eventual use.
	storageAddr := fmt.Sprintf("%s:%s", distr.Config.Storage.PublicIP, distr.Config.Storage.MailPort)

	// Establish TLS connection to worker.
	conn, err := InternalConnect(fmt.Sprintf("%s:%s", workerIP, workerPort), distr.IntlTLSConfig, distr.Config.IntlConnRetry, true, storageAddr)
	if err != nil {
		c.Error("Internal connection failure", err)
		return false
	}

	// Save context to connection.
	c.OutConn = conn
	c.OutReader = bufio.NewReader(conn)
	c.IntlTLSConfig = distr.IntlTLSConfig
	c.IntlConnRetry = distr.Config.IntlConnRetry
	c.ClientID = clientID
	c.UserName = userCredentials[0]

	// Inform worker node about which session just started.
	err = c.SignalSessionStart(false, true, storageAddr)
	if err != nil {
		c.Error("Encountered send error when distributor was signalling context to worker", err)
		return false
	}

	// Signal success to client.
	err = c.Send(true, fmt.Sprintf("%s OK LOGIN completed", req.Tag))
	if err != nil {
		c.Error("Encountered send error", err)
		return false
	}

	return true
}

// Proxy forwards one request between the distributor
// node and the responsible worker node.
func (distr *Distributor) Proxy(c *Connection, rawReq string) bool {

	// Pass message to worker node.
	err := c.InternalSend(false, rawReq, true, fmt.Sprintf("%s:%s", distr.Config.Storage.PublicIP, distr.Config.Storage.MailPort))
	if err != nil {
		c.Error("Could not proxy request to worker", err)
		return false
	}

	// Reserve space for answer buffer.
	bufResp := make([]string, 0, 6)

	// Receive incoming worker response.
	curResp, err := c.InternalReceive(false)
	if err != nil {
		c.Error("Failed to receive worker's response to proxied command", err)
		return false
	}

	// As long as the responsible worker has not
	// indicated the end of the current operation,
	// continue to buffer answers.
	for (curResp != "> done <") && (strings.HasPrefix(curResp, "> literal: ") != true) {

		// Append it to answer buffer.
		bufResp = append(bufResp, curResp)

		// Receive incoming worker response.
		curResp, err = c.InternalReceive(false)
		if err != nil {
			c.Error("Encountered receive error from worker", err)
			return false
		}
	}

	for i := range bufResp {

		// Send all buffered worker answers to client.
		err = c.Send(true, bufResp[i])
		if err != nil {
			c.Error("Encountered send error to client", err)
			return false
		}
	}

	// Special case: We expect literal data in form of a
	// RFC defined mail message.
	if strings.HasPrefix(curResp, "> literal: ") {

		// Strip off left and right elements of signal.
		// This leaves the awaited amount of bytes.
		numBytesString := strings.TrimPrefix(curResp, "> literal: ")
		numBytesString = strings.TrimSuffix(numBytesString, " <")

		// Convert string amount to int.
		numBytes, err := strconv.Atoi(numBytesString)
		if err != nil {
			c.Error("Encountered conversion error for string to int", err)
			return false
		}

		// Reserve space for exact amount of expected data.
		msgBuffer := make([]byte, numBytes)

		// Read in that amount from connection to client.
		_, err = io.ReadFull(c.IncReader, msgBuffer)
		if err != nil {
			c.Error("Encountered error while reading client literal data", err)
			return false
		}

		// Pass on data to worker. Mails have to be ended by
		// newline symbol.
		_, err = fmt.Fprintf(c.OutConn, "%s", msgBuffer)
		if err != nil {
			c.Error("Encountered passing send error to worker", err)
			return false
		}

		// Reserve space for answer buffer.
		bufResp := make([]string, 0, 6)

		// Receive incoming worker response.
		curResp, err := c.InternalReceive(false)
		if err != nil {
			c.Error("Encountered receive error from worker after literal data was sent", err)
			return false
		}

		// As long as the responsible worker has not
		// indicated the end of the current operation,
		// continue to buffer answers.
		for curResp != "> done <" {

			// Append it to answer buffer.
			bufResp = append(bufResp, curResp)

			// Receive incoming worker response.
			curResp, err = c.InternalReceive(false)
			if err != nil {
				c.Error("Encountered receive error from worker after literal data was sent", err)
				return false
			}
		}

		for i := range bufResp {

			// Send all buffered worker answers to client.
			err = c.Send(true, bufResp[i])
			if err != nil {
				c.Error("Encountered send error to client after literal data was sent", err)
				return false
			}
		}
	}

	return true
}
