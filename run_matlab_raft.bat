rem echo off
set PATH=C:\Program Files\Matlab\R2016a\bin\win64;C:\TDM-GCC-64\bin;%PATH%
rem cd %GOPATH%
go build -o clientbayes.exe .\clientMatlab_raft\clientM_raft.go
go build -o serverbayes.exe .\serverMatlab_raft\serverM_raft.go
pause
start cmd /c .\serverbayes.exe 127.0.0.1:2600 raftlist.txt 1 s1
timeout 3
start cmd /c .\serverbayes.exe 127.0.0.1:2601 raftlist.txt 2 s2
start cmd /c .\serverbayes.exe 127.0.0.1:2602 raftlist.txt 3 s3
start cmd /c .\serverbayes.exe 127.0.0.1:2603 raftlist.txt 4 s4
start cmd /c .\serverbayes.exe 127.0.0.1:2604 raftlist.txt 5 s5
rem timeout 5
pause
start cmd /K .\clientbayes.exe hospital1 127.0.0.1:2605 serverlist.txt %GOPATH%\src\github.com\4180122\distbayes\testdata\x1.txt %GOPATH%\src\github.com\4180122\distbayes\testdata\y1.txt h1 &
start cmd /K .\clientbayes.exe hospital2 127.0.0.1:2606 serverlist.txt %GOPATH%\src\github.com\4180122\distbayes\testdata\x2.txt %GOPATH%\src\github.com\4180122\distbayes\testdata\y2.txt h2 &
rem start cmd /c .\clientbayes.exe hospital3 127.0.0.1:2607 serverlist.txt testdata\x3.txt testdata\y3.txt h3 &
rem start cmd /c .\clientbayes.exe hospital4 127.0.0.1:2608 serverlist.txt testdata\x4.txt testdata\y4.txt h4 &
rem start cmd /c .\clientbayes.exe hospital5 127.0.0.1:2609 serverlist.txt testdata\x5.txt testdata\y5.txt h5 &
