# Contributing to Haaukins 

If you wish to make contribution to Haaukins platform, you are very welcome to do it, any contribution would be beneficial. 

## Pull requests are always welcome

We are always thrilled to receive pull requests, and do our best to process them as quickly as possible. Not sure if that typo is worth a pull request? Do it! We will appreciate it.

If your pull request is not accepted on the first try, don't be discouraged! If there's a problem with the implementation, hopefully you received feedback on what to improve.

## Contributing Rules 


In order to contribute Haaukins, you can fork the repo and make your changes on your fork in a feature branch. 
Haaukins developments is based on [Git Feature Branch Workflow](https://www.atlassian.com/git/tutorials/comparing-workflows/feature-branch-workflow)

- For bug fixes and feature requests, the name of branch that you will work on should follow the convention [XXXX]-something, XXXX refers to issue number. 

- There should be a naming convention (function names and variable names should follow defined pattern such as getName(private) or GetName (public).

- Use gofmt to ensure code formatting that is easy to read.

- Commit messages should start with capitalized letter and follow short summary of the changes. Commits which are fixing or closing an issue should include reference to it, such as `Closes #[XXXX]` or `Fixes #[XXXX]` 

- Before submit your changes and having a pull request, make sure that all tests passed. 

- Have descriptive comments on changes.

### Code Review Process 

- All changes to the master branch must be through Pull Requests, and all PRs must be approved by at least one maintainer, who must be different from the creator of the PR. 

- Pull requests should be made into version branch instead of master, since there will be a next version branch when previous release took place, for instance if a patch version is released recently called 1.6.3, then new branch 1.6.4 will be automatically created and it will be identical to 1.6.3. So when a new feature branch created, pull requests should take branch 1.6.4 as base target branch. 

## Create Issue... 

- Creating an issue on Haaukins development process should follow given templates, although there is a chance to create a pure issue without templating,  it is required to give priority to templates which are exists on repo.


#### Feature Request Template

        > Is your feature request related to a problem? Please describe.**
        A clear and concise description of what the problem is. Ex. I'm always frustrated when [...]

        > Describe the solution you'd like**
        A clear and concise description of what you want to happen.

        > Describe alternatives you've considered
        A clear and concise description of any alternative solutions or features you've considered.

        > Additional context
        Add any other context or screenshots about the feature request here.


#### Bug Fix Template

       > Describe the bug
        A clear and concise description of what the bug is.

       > To Reproduce
          Steps to reproduce the behavior:
          1. Go to '...'
          2. Click on '....'
          3. Scroll down to '....'
          4. See error

       > Expected behavior
        A clear and concise description of what you expected to happen.

       > Screenshots
        If applicable, add screenshots to help explain your problem.

       > Desktop (please complete the following information):
         - OS: [e.g. iOS]
         - Browser [e.g. chrome, safari]
         - Version [e.g. 22]

       > Smartphone (please complete the following information):
         - Device: [e.g. iPhone6]
         - OS: [e.g. iOS8.1]
         - Browser [e.g. stock browser, safari]
         - Version [e.g. 22]

       > Additional context
        Add any other context about the problem here.
        
## Check for existing issues

- It would be very practical for contributers to have a look for existing issues, if the issue is already exist on [Issues](https://github.com/aau-network-security/haaukins/issues) tab, then giving +1 to the issue will make the issue's priority higher, so that we can give full focus on it. 


 
