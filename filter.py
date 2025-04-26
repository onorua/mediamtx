import re
import sys
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
from datetime import datetime

if len(sys.argv) < 3:
    print("Usage: python filter.py <filename> <fields>")
    sys.exit(1)

filename = sys.argv[1]
fields = sys.argv[2:]

timestamps = []
field_values = {field: [] for field in fields}

with open(filename, "r") as f:
    for line in f:
        if "SRT Stats" not in line:
            continue

        timestamp = re.search(r"\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}", line).group()
        timestamps.append(datetime.strptime(timestamp, "%Y/%m/%d %H:%M:%S"))
        extracted_values = {"timestamp": timestamp}

        for field in fields:
            match = re.search(rf"{field}:(\d+\.?\d*)", line)
            value = float(match.group(1)) if match else 0.0
            extracted_values[field] = value
            field_values[field].append(value)

        print(" | ".join(f"{key}={value}" for key, value in extracted_values.items()))

# Plot multiple graphs
fig, axes = plt.subplots(len(fields), 1, figsize=(12, 6 * len(fields)), sharex=True)

# Start the plot window in fullscreen mode
manager = plt.get_current_fig_manager()
manager.full_screen_toggle()

for i, field in enumerate(fields):
    axes[i].plot(timestamps, field_values[field], label=field)  # Removed marker='o'
    axes[i].set_ylabel(field)
    axes[i].legend(loc="upper left")
    axes[i].grid(True)

# Format x-axis for timestamps
axes[-1].set_xlabel("Timestamp")
axes[-1].xaxis.set_major_formatter(mdates.DateFormatter("%Y-%m-%d %H:%M:%S"))
interval = max(1, len(timestamps) // 10)  # Dynamically adjust interval
axes[-1].xaxis.set_major_locator(mdates.AutoDateLocator(interval_multiples=True))
plt.setp(axes[-1].xaxis.get_majorticklabels(), rotation=45, ha="right")

plt.tight_layout()
plt.show()
