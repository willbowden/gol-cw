import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns

data = pd.read_csv('results.csv', header=0, names=['name', 'time', 'range'])
# convert seconds from ns 
data['time'] /= 1e+9

data['threads'] = data['name'].str.extract('Gol/(\d+)x\d+x\d+-\d+-\d+').apply(pd.to_numeric)
data['cpu_cores'] = data['name'].str.extract('Gol/\d+x\d+x\d+-(\d+)-\d+').apply(pd.to_numeric)

print(data)

# Plot a bar chart.
ax = sns.barplot(data=data, x='threads', y='time', hue='cpu_cores', ci=None)

# Set descriptive axis labels.
ax.set(xlabel='Worker threads used', ylabel='Time taken (s)')

# added legend
ax.legend(title='CPU Cores')

# Display the full figure.
plt.show()
