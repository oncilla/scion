#!/usr/bin/env python3

import sys
import json
import seaborn as sns
import matplotlib.pyplot as plt
import pandas as pd

border = 0.25


def loadData(fname):
    with open(fname) as f:
        data = json.load(f)
    d = {"vals": data["vals"],
         "labels": data["labels"],
         }
    d = pd.DataFrame(data=d)
    return data, d


def plotFig(fn):
    js, d = loadData(fn)
    title = js["title"]
    label = js["label"]
    runs = js["runs"]

    sns.set(style="darkgrid")
    # Initialize the figure with a logarithmic x axis
    f, ax = plt.subplots(figsize=(3 + 1.1*runs, 5))
    sns.set_context("paper")
    # ax.set_xscale("log")

    # Plot the orbital period with horizontal boxes
    sns.boxplot(y="vals", x="labels", data=d,
                whis="range", palette="vlag", width=0.6)

    # Add in points to show each observation
    sns.swarmplot(y="vals", x="labels", data=d,
                  size=2, color=[0.2,0.25,0.2], linewidth=0)

    # Tweak the visual presentation
    ax.yaxis.grid(True)
    ax.set(xlabel="", ylabel="Duration (µs)")
    min_val = d["vals"].min()
    max_val = d["vals"].max()
    diff = max_val - min_val
    br = diff * border
    ax.set_ylim([min_val-br, max_val+br])
    print([min_val-br, max_val+br])
    # sns.despine(trim=True, left=True)
    plt.tight_layout()
    plt.savefig(fn.replace("json", "eps"), format="eps")
    plt.savefig(fn.replace("json", "png"), format="png")
    plt.show()


def main():
    files = sys.argv[1:]
    for f in files:
        plotFig(f)


if __name__ == "__main__":
    main()
