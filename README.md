# sphinx-hq-challenge
The challenge is to send as many morties as possible to Jessica planet.

[link](https://challenge.sphinxhq.com/)


## SOLUTION 1

Send 2 Morties to each planet.  
If the Morties survive, send **1 more** on the next request  
If the Morties don't survive, send **1 less** on the next request  

### Result
| Rescued | Lost | Rate  |
|:-------:|:----:|:-----:|
|   526   | 474  | 53.6% |

### Problem with this method

This solution only considers the previous action irrespective of how many there were.  
This approach is too naive as a single action is not enough information. We do not know the standard deviation of the survival rate of the planets

---

## Solution 2

Update solution 1 to consider the overall survival rate of a planet by taking the  
Morties that were sent to a planet and survived over the total morties sent to the planet  
Conditions:  
0 < ratio <= 33%. Send 1 morty  
33 < ratio <= 66%. Send 2 morties  
66 < ratio <= 100%. Send 2 morties  

Update: I tried to compensate for the standard deviation by settig the ranges between 46% and 54% but that gave the same result as above.

### Result
| Rescued | Lost | Rate  |
|:-------:|:----:|:-----:|
|   548   | 468  | 54.8% |

### Problem with this method

This method does not consider the standard deviation of the survivability of the each planet.
This method further ignores the impact of losing 1 morty vs 3 morties has on a planet's survivability.

## Solution 3

I started searching online and found that these problems are called the multi-arm bandit problems.  
After some more searching I found [this white paper](https://www.cs.mcgill.ca/~vkules/bandits.pdf)
In the use case of the clinical trials the paper recommened the _Epsilon Greedy_ algorithm as the most optimal solution. 
And seeing as I know nothing, I'll be implementing the _Epsilon Greedy_ algorithm here

#### My understanding of Epsilon Greedy

You select an epsilon, which is the essentially the ratio between exploring and exploiting  
Then you give the algorithm a random number **P**:  

_if_ the **P** < Epsilon _then_ do a random action 
_else_ choose the current best action

I am defining an action as the combination of how many morties are sent to each planet and the survival rate of each action

### Result
| Rescued | Lost | Rate  |
|:-------:|:----:|:-----:|
|   520   | 480  |  52%  |

### Problem with the method (applied here)

Defining the winning action.  
What I went with was to choose the combination of morties sent to each planet against the average survival rate and the highest survival rate won.


