/*
 Copyright 2011 The Go Authors.  All rights reserved.
 Use of this source code is governed by a BSD-style
 license that can be found in the LICENSE file.
*/

/* the response from /api/groups should look like:
    {
        "Groups": [{
            "Name": "GoSV",
            "URL": "http://www.meetup.com/golangsv",
            "Members": 194,
            "City": "San Mateo",
            "Country": "US"
        }, {
            "Name": "GoSF",
            "URL": "http: //www.meetup.com/golangsf",
            "Members": 1393,
            "City": "San Francisco",
            "Country": "US"
        }],
        "Errors": [
            "something bad happened"
        ]
    }
*/
function GroupsCtrl($scope, $http, $filter) {
    'use strict';

    $scope.groups = [];
    $scope.errors = [];
    $scope.search = {};
    $scope.filteredFields = ['Name', 'City', 'Country', 'Continent'];
    $scope.filteredGroups = [];

    $scope.refilter = function() {
        for (var f in $scope.filteredFields) {
            var field = $scope.filteredFields[f];
            if ($scope.search[field] === '') delete $scope.search[field];
        }
        $scope.filteredGroups = $filter('filter')($scope.groups, $scope.search);
    };

    $scope.totalSum = function() {
        var n = 0;
        for (var i in $scope.filteredGroups) n += $scope.filteredGroups[i].Members;
        return n;
    };

    $http.get('/api/groups').then(function(res) {
        $scope.groups = res.data.Groups;
        for (var i in res.data.Errors) {
            $scope.log(res.data.Errors[i]);
        }
        $scope.refilter();
    }, function(msg) {
        $scope.log(msg.data);
    });

    $scope.log = function(msg) {
        $scope.errors.push(msg);
    };
}
