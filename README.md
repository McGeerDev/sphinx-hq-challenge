# sphinx-hq-challenge
The challenge is to send as many morties as possible to Jessica planet.

[link](https://challenge.sphinxhq.com/)


## SOLUTION 1

Send 2 Morties to each planet.  
If the Morties survive, send **1 more** on the next request  
If the Morties don't survive, send **1 less** on the next request  

### Result
|Rescued|Lost|Rate|
|:---:|:---:|:---:|
| 526 | 474 | 53% |

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
