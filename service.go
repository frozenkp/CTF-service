package main

import(
  "log"
  "flag"
  "net"
  "os/exec"
  "os/user"
  "strconv"
  "fmt"
  "syscall"
  "io"
  "context"
  "time"
)

type streamResult struct {
  err     error
  remote  bool
}

func main(){
  // flag
  configFile := flag.String("c", "", "Config File (yaml)")
  flag.Parse()

  if *configFile == "" {
    log.Fatal("No config file.")
  }

  // read config
  config, err := readConfig(*configFile)
  if err != nil {
    log.Fatal(err)
  }

  // service
  server, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
  if err != nil {
    log.Fatalf("Fail to start server, %s", err.Error())
  }

  log.Println("Start service on port", config.Port)

  for true {
    conn, err := server.Accept()
    if err != nil || conn == nil {
      log.Println("Fail to connect, %s", err)
      continue
    }
    log.Println("Connected from", conn.RemoteAddr())

    go func(){
      err, closed := process(conn, config)
      if err != nil {
        log.Printf("%s: %s\n", conn.RemoteAddr(), err.Error())
      }
      if !closed {
        conn.Close()
      }
      log.Printf("Connection from %s disconnected.\n", conn.RemoteAddr())
    }()
  }
}

func process(conn net.Conn, config Config) (error, bool) {
  // exec
  var cmd *exec.Cmd
  if config.Timeout < 0 {
    cmd = exec.Command(config.Command)
  } else {
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Millisecond)
    defer cancel()
    cmd = exec.CommandContext(ctx, config.Command)
  }

  // set user
  userInfo, err := user.Lookup(config.User)
  if err != nil {
    return err, false
  }
  uid, err := strconv.ParseUint(userInfo.Uid, 10, 32)
  if err != nil {
    return err, false
  }
  gid, err := strconv.ParseUint(userInfo.Gid, 10, 32)
  if err != nil {
    return err, false
  }

  cmd.SysProcAttr = &syscall.SysProcAttr{}
  cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}

  // bind reader, writer
  stdin, err := cmd.StdinPipe()
  if err != nil {
    return err, false
  }

  stdout, err := cmd.StdoutPipe()
  if err != nil {
    return err, false
  }

  resch := make(chan streamResult, 1)
  go stream_copy(stdin, conn, resch, true)
  go stream_copy(conn, stdout, resch, false)

  // start
  err = cmd.Start()
  if err != nil {
    return err, false
  }

  // wait
  var retErr error = nil
  for i:=0; i<2; i++ {
    res := <-resch
    if res.remote {
      // close process input / output
      stdin.Close()
      stdout.Close()
      // kill process
      if cmd.Process != nil {
        cmd.Process.Kill()
      }
      cmd.Wait()
    } else {
      // close connection
      conn.Close()
    }

    // save err
    if i == 0 {
      retErr = res.err
    }
  }

  // close channel
  close(resch)

  return retErr, true
}

// modified from https://github.com/vfedoroff/go-netcat/blob/master/main.go
func stream_copy(dst io.Writer, src io.Reader, resch chan streamResult, remote bool){
  buf := make([]byte, 1024)
  for {
    // read and write
    var nBytes int
    var err error
    nBytes, err = src.Read(buf)
    if err != nil {
      if err != io.EOF {
        resch <- streamResult{err, remote}
      } else {
        resch <- streamResult{nil, remote}
      }
      break
    }
    _, err = dst.Write(buf[0:nBytes])
    if err != nil {
      resch <- streamResult{err, remote}
      break
    }
  }
}

