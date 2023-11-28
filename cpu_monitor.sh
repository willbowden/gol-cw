#!/bin/bash

outputFile="cpu_usage.csv"

get_core_count() {
  grep -c ^processor /proc/cpuinfo
}

get_cpu_usage() {
  echo -n "$(date +%T)," >> "$outputFile"
  mpstat -P ALL 1 1 | awk 'NR>4 {printf "%.2f,", ($3+$5)}' | sed 's/.$//' >> "$outputFile"
  echo "" >> "$outputFile"
}

coreCount=$(get_core_count)

header="time"
for i in $(seq 1 $coreCount); do
  header+=",core_$i"
done
echo "$header" > "$outputFile"

echo "Starting CPU monitoring..."
while true; do
  get_cpu_usage
  sleep 5
done
