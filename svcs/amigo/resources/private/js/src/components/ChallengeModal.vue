<template>
    <div>
        <b-modal ref="modal" id="challengeModal" centered hide-footer hide-header>
            <div class="modal-body">
                <button type="button" class="close" v-on:click="$bvModal.hide('challengeModal')">
                    <span aria-hidden="true">×</span>
                </button>
                <nav>
                    <div class="nav nav-tabs" id="nav-tab" role="tablist">
                        <a class="nav-item nav-link active" id="nav-challenge-tab" data-toggle="tab" href="#nav-challenge" role="tab" aria-controls="nav-challenge" aria-selected="true">Challenge</a>
                        <a class="nav-item nav-link" id="nav-solves-tab" data-toggle="tab" href="#nav-solves" role="tab" aria-controls="nav-solves" aria-selected="false">1 Solves</a>
                    </div>
                </nav>
                <div class="tab-content" id="nav-tabContent">
                    <div class="tab-pane fade show active" id="nav-challenge" role="tabpanel" aria-labelledby="nav-challenge-tab">
                        <h2 class="chal-name text-center pt-3">{{challenge.name}}</h2>
                        <h3 class="chal-value text-center">{{challenge.points}}</h3>
                        <span class="chal-desc">
                            <p>{{challenge.description}}</p>
                        </span>
                        <form ref="form" @submit.stop.prevent="submitFlag()">
                            <div class="row submit-row">
                                <div class="col-md-9 form-group">
                                    <input class="form-control" type="text" name="answer" id="answer-input" placeholder="Flag">
                                    <input id="chal-id" type="hidden" value="11">
                                </div>
                                <div class="col-md-3 form-group key-submit">
                                    <button type="submit" id="submit-key" tabindex="5" class="btn btn-md btn-outline-secondary float-right">Submit</button>
                                </div>
                            </div>
                        </form>
                        <div class="text-center text-danger">
                            {{submitFlagError}}
                        </div>
                    </div>
                    <div class="tab-pane fade" id="nav-solves" role="tabpanel" aria-labelledby="nav-solves-tab">
                        <table class="table table-striped text-center">
                            <thead>
                            <tr>
                                <td><b>Name</b>
                                </td>
                                <td><b>Date</b>
                                </td>
                            </tr>
                            </thead>
                            <tbody id="bhooo">
                                <tr>
                                    <td><a href="#">Test</a></td>
                                    <td>7 days ago</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </b-modal>
    </div>
</template>

<script>
    export default {
        name: "ChallengeModal",
        props: {
            challenge: Object
        },
        data: function (){
            return{
                submitFlagError: null
            }
        },
        methods: {
            submitFlag: function () {
                this.submitFlagError = "Incorret Flag";
                window.console.log("inside submit Flag")
            }
        }
    }
</script>

<style scoped>

</style>