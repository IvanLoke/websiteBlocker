#!/bin/bash
ps aux | grep selfcontrol | awk '{print $2}' | xargs kill -9