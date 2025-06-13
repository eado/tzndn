#!/bin/bash

# Testbed URLs
testbeds=(
    "suns.cs.ucla.edu"
)

# Output CSV file
output_file="test_results.csv"

# Create CSV header
echo "testbed_url,total_time_seconds" > "$output_file"

# Function to run a single test
run_test() {
    local testbed=$1
    local run_number=$2
    
    start_time=$(date +%s.%N)
    ./consumer main test@eado.me "${testbed}:6363" > /dev/null 2>&1
    end_time=$(date +%s.%N)
    
    # Calculate duration
    duration=$(echo "$end_time - $start_time" | bc -l)
    
    # Append directly to CSV
    echo "${testbed},${duration}" >> "$output_file"
    
    echo "Completed run $run_number for $testbed in ${duration}s"
}

# Export function so it can be used by parallel processes
export -f run_test
export output_file

echo "Starting 20 concurrent tests for suns.cs.ucla.edu..."
echo "Results will be saved to $output_file"

# Run all tests concurrently
for testbed in "${testbeds[@]}"; do
    for i in {1..20}; do
        run_test "$testbed" "$i" &
    done
done

# Wait for all background processes to complete
wait

echo "All tests completed. Results saved to $output_file"

# Clean up lock file
rm -f "$output_file.lock"

# Show summary
echo ""
echo "Summary:"
echo "Total runs: $(tail -n +2 "$output_file" | wc -l)"
echo "Average time per testbed:"
for testbed in "${testbeds[@]}"; do
    avg_time=$(grep "$testbed" "$output_file" | cut -d',' -f2 | awk '{sum+=$1; count++} END {if(count>0) print sum/count; else print 0}')
    echo "  $testbed: ${avg_time}s"
done