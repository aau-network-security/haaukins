<template>
    <div>
        <Plotly :data="traces" :layout="layout"></Plotly>
        {{dummy}}
    </div>
</template>

<script>
    import { Plotly } from 'vue-plotly'

    export default {
        name: "ScoreGraph",
        components: {
            Plotly
        },
        data: function () {
            return {
                dummy: [
                    { "id": "227e9de3", "name": "test", "tpoints": 22, "completions": [ "2020-03-14T12:30:15.480666371+01:00", "2020-03-14T12:30:35.295348636+01:00" ], "points": [ 7, 15 ], "is_user": false },
                    { "id": "dd633565", "name": "menne", "tpoints": 0, "completions": [ "2020-03-13T12:30:15.480666371+01:00", null ], "points": [ 7, 15 ], "is_user": false },
                    { "id": "4e10b5c1", "name": "merlo", "tpoints": 0, "completions": [ null, "2020-03-12T12:30:15.480666371+01:00" ], "points": [ 7, 15 ], "is_user": true } ],
                data: [{
                    x: [1, 2, 3, 4],
                    y: [10, 15, 13, 17],
                    type: "scatter"
                }],
                traces: [],
                layout: {
                    title: 'Score Graph',
                    paper_bgcolor: 'rgba(0,0,0,0)',
                    plot_bgcolor: 'rgba(0,0,0,0)',
                    hovermode: 'closest',
                    xaxis: {
                        showgrid: false,
                        showspikes: true,
                    },
                    yaxis: {
                        showgrid: false,
                        showspikes: true,
                    },
                    legend: {
                        "orientation": "h"
                    }
                }
            }
        },
        created() {
            this.sortSolvedData()
            this.scoreGraph()
        },
        methods: {
            sortSolvedData: function(){
                for (let i=0; i < this.dummy.length; i++){

                    let list = [];
                    for (let j = 0; j < this.dummy[i].completions.length; j++)
                        list.push({'date': this.dummy[i].completions[j], 'points': this.dummy[i].points[j]});

                    list.sort((a, b) => new Date(a.date) - new Date(b.date) )

                    for (let k = 0; k < list.length; k++) {
                        this.dummy[i].completions[k] = list[k].date;
                        this.dummy[i].points[k] = list[k].points;
                    }
                }
            },
            scoreGraph: function () {

                for (let i=0; i < this.dummy.length; i++){
                    let team_score = [];
                    let times = [];
                    for(let j = 0; j < this.dummy[i].completions.length; j++){
                        if (this.dummy[i].completions[j] != null){
                            let date = new Date(this.dummy[i].completions[j])
                            times.push(date)
                            team_score.push(this.dummy[i].points[j])
                        }
                    }

                    team_score = this.cumulativeSum(team_score);
                    let trace = {
                        x: times,
                        y: team_score,
                        mode: 'lines+markers',
                        name: this.dummy[i].name,
                        marker: {
                            color: this.colorHash(this.dummy[i].name + this.dummy[i].id),
                        },
                        line: {
                            color: this.colorHash(this.dummy[i].name + this.dummy[i].id),
                        }
                    };
                    this.traces.push(trace);
                }

                this.traces.sort(function(a, b) {
                    var scorediff = b['y'][b['y'].length - 1] - a['y'][a['y'].length - 1];
                    if(!scorediff) {
                        return a['x'][a['x'].length - 1] - b['x'][b['x'].length - 1];
                    }
                    return scorediff;
                });
                window.console.log(this.traces);


            },
            cumulativeSum: function (arr) {
                let result = arr.concat();
                for (let i = 0; i < arr.length; i++){
                    result[i] = arr.slice(0, i + 1).reduce(function(p, i){ return p + i; });
                }
                return result
            },
            colorHash: function (str) {
                let hash = 0;
                for (let i = 0; i < str.length; i++) {
                    hash = str.charCodeAt(i) + ((hash << 5) - hash);
                }
                let colour = '#';
                for (let j = 0; j < 3; j++) {
                    let value = (hash >> (j * 8)) & 0xFF;
                    colour += ('00' + value.toString(16)).substr(-2);
                }
                return colour;
            }
        }
    }
</script>

<style scoped>

</style>