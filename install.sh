#!/bin/bash
systemctl stop SMA_Watch
cp bin/ARM/SMA_Watch /usr/bin
systemctl start SMA_Watch
