package main

import (
        "fmt"
        "github.com/pkg/xattr"
        "golang.org/x/crypto/ssh/terminal"
        "log"
        "os"
        "os/exec"
        "strconv"
        "strings"
        "math"
)

type Quota struct {
        bytes int
        bhard int
        bsoft int
        files int
        fhard int
        fsoft int
}

func cephQuota(name string, path string) Quota {
        var q Quota
        var err error

        fullpath := path + "/" + name

        // Quota values
        bhard, err := xattr.Get(fullpath, "ceph.quota.max_bytes")
        if err != nil {
                q.bhard = -1
        } else {
                // converting to a string first seems hackish and gross
                q.bhard, _ = strconv.Atoi(string(bhard))
        }
        fhard, err := xattr.Get(fullpath, "ceph.quota.max_files")
        if err != nil {
                q.fhard = -1
        } else {
                // converting to a string first seems hackish and gross
                q.fhard, _ = strconv.Atoi(string(fhard))
        }

        // Current values
        bytes, err := xattr.Get(fullpath, "ceph.dir.rbytes")
        if err != nil {
                q.bytes = -1
        } else {
                q.bytes, _ = strconv.Atoi(string(bytes))
        }
        files, err := xattr.Get(fullpath, "ceph.dir.rfiles")
        if err != nil {
                log.Println("Could not get Ceph files")
                q.bytes = -1
        } else {
                q.files, _ = strconv.Atoi(string(files))
        }

        // Ceph doesn't have soft quotas, so we just set it to the same as hard
        q.bsoft = q.bhard
        q.fsoft = q.fhard

        return q
}

func xfsQuota(name string, path string) Quota {
        var q Quota
        var err error

        // just shell out. it'll be fine (tm)
        out, err := exec.Command("/bin/quota", "-w", "--hide-device", "-p", "-f", path).Output()
        if err != nil {
                log.Fatal(err)
        }

        // nope doesnt seem brittle at all
        s := strings.Split(string(out), "\n")
        if len(s) != 4 {
            // user probably doesn't have a quota. set everything to -1 and
            // we'll deal with it later
            q = Quota{-1,-1,-1,-1,-1,-1}
            return q
        }
        sf := strings.Fields(s[2])

        // sf = [blocks,bsoft,bhard,bgrace,files,fsoft,fhard,fgrace]
        // quota reports in 1K blocks for historical reasons
        kbytes, _ := strconv.Atoi(sf[0])
        kbsoft, _ := strconv.Atoi(sf[1])
        kbhard, _ := strconv.Atoi(sf[2])
        q.bytes = kbytes * 1024
        q.bsoft = kbsoft * 1024
        q.bhard = kbhard * 1024

        q.files, _ = strconv.Atoi(sf[4])
        q.fsoft, _ = strconv.Atoi(sf[5])
        q.fhard, _ = strconv.Atoi(sf[6])

        return q

}

func utilizationBar (q *Quota) {
    // get some terminal info to make rad status bars
    w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
    if err != nil {
        // we'll make a sensible guess
        w = 80
    }

    maxwidth := w/4
    if q.bytes == -1 || q.bsoft == -1 || q.bhard == -1 {
        msg:="No quota information! Please contact user-support@opensciencegrid.org"
        fmt.Printf("[ %s", msg)
        for i := 1; i <= (maxwidth - len(msg)); i++ {
          fmt.Printf(" ")
        }
        fmt.Printf(" ]\n")
    } else {
        quotaratio := float64(q.bytes)/float64(q.bsoft)

        markers := math.Ceil(float64(maxwidth) * quotaratio)

        fmt.Printf("[ ")
        for i := 1; i <= int(maxwidth); i++ {
            if i <= int(markers) {
              fmt.Printf("#")
            } else {
              fmt.Printf(" ")
            }
        }
        fmt.Printf(" ]")

        quotapct := int(math.Ceil(quotaratio * 100))
        fmt.Printf(" %d%%", quotapct) 
        fmt.Printf(" (%d/%d MB)\n", q.bytes / 1000 / 1000, q.bsoft / 1000 /1000)
    }

    return
}


func main() {
        username := os.Getenv("USER")

        cq := cephQuota(username, "/public")
        xq := xfsQuota(username, "/home")

        fmt.Printf("Disk utilization for %s:\n", username)
        fmt.Printf("/home:   ")
        utilizationBar(&xq)
        fmt.Printf("/public: " )
        utilizationBar(&cq)
}

